package ssh

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// SecretKind distinguishes login passwords from key passphrases.
// OpenSSH uses the same askpass program for both; the prompt text tells them apart.
type SecretKind string

const (
	SecretPassword   SecretKind = "password"
	SecretPassphrase SecretKind = "passphrase"
)

// PromptRequest is what OpenSSH asked for via SSH_ASKPASS.
type PromptRequest struct {
	HostID string
	Prompt string
	Kind   SecretKind
}

// Prompter returns a secret for an SSH_ASKPASS request. It may be called from a
// background goroutine and should respect ctx cancellation.
type Prompter func(ctx context.Context, req PromptRequest) (string, error)

// ErrPromptCancelled is returned when the user dismisses a password prompt.
var ErrPromptCancelled = errors.New("password prompt cancelled")

// KindFromPrompt classifies an OpenSSH askpass prompt.
func KindFromPrompt(prompt string) SecretKind {
	if strings.Contains(strings.ToLower(prompt), "passphrase") {
		return SecretPassphrase
	}
	return SecretPassword
}

type secretStore struct {
	mu              sync.Mutex
	password        string
	passphrase      string
	byPrompt        map[string]string
	used            map[string]bool
	promptCancelled bool
}

func newSecretStore() *secretStore {
	return &secretStore{
		byPrompt: make(map[string]string),
		used:     make(map[string]bool),
	}
}

func (s *secretStore) setPassword(password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.password = password
}

func (s *secretStore) setPassphrase(passphrase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passphrase = passphrase
}

func (s *secretStore) nextAttempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.used = make(map[string]bool)
	s.promptCancelled = false
}

func (s *secretStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.password = ""
	s.passphrase = ""
	s.byPrompt = make(map[string]string)
	s.used = make(map[string]bool)
}

func (s *secretStore) cancelled() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.promptCancelled
}

func (s *secretStore) respond(ctx context.Context, req PromptRequest, prompter Prompter) (string, error) {
	if s == nil {
		if prompter == nil {
			return "", errors.New("no password available")
		}
		return prompter(ctx, req)
	}

	s.mu.Lock()
	if s.promptCancelled {
		s.mu.Unlock()
		return "", ErrPromptCancelled
	}
	if secret, ok := s.takeLocked(req); ok {
		s.mu.Unlock()
		return secret, nil
	}
	s.mu.Unlock()

	if prompter == nil {
		return "", errors.New("no password available")
	}
	secret, err := prompter(ctx, req)
	if err != nil {
		if errors.Is(err, ErrPromptCancelled) || errors.Is(err, context.Canceled) {
			s.mu.Lock()
			s.promptCancelled = true
			s.mu.Unlock()
			return "", ErrPromptCancelled
		}
		return "", err
	}

	s.mu.Lock()
	s.rememberLocked(req, secret)
	s.mu.Unlock()
	return secret, nil
}

func (s *secretStore) takeLocked(req PromptRequest) (string, bool) {
	if s.used[req.Prompt] {
		// ssh is asking again, so the previous secret was rejected.
		delete(s.byPrompt, req.Prompt)
		if req.Kind == SecretPassphrase {
			s.passphrase = ""
		} else {
			s.password = ""
		}
		return "", false
	}
	if secret := s.byPrompt[req.Prompt]; secret != "" {
		s.used[req.Prompt] = true
		return secret, true
	}
	switch req.Kind {
	case SecretPassphrase:
		if s.passphrase != "" {
			s.used[req.Prompt] = true
			s.byPrompt[req.Prompt] = s.passphrase
			return s.passphrase, true
		}
	default:
		if s.password != "" {
			s.used[req.Prompt] = true
			s.byPrompt[req.Prompt] = s.password
			return s.password, true
		}
	}
	return "", false
}

func (s *secretStore) rememberLocked(req PromptRequest, secret string) {
	if s.byPrompt == nil {
		s.byPrompt = make(map[string]string)
	}
	if s.used == nil {
		s.used = make(map[string]bool)
	}
	s.byPrompt[req.Prompt] = secret
	s.used[req.Prompt] = true
	if req.Kind == SecretPassphrase {
		s.passphrase = secret
	} else {
		s.password = secret
	}
}
