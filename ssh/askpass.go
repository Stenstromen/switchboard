package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// AskpassEnableEnv is set on the ssh child so this same binary, when
	// invoked as SSH_ASKPASS, knows to answer a prompt instead of starting
	// the CLI.
	AskpassEnableEnv = "SWITCHBOARD_ASKPASS"

	// AskpassSockEnv is the unix socket used by the askpass helper.
	AskpassSockEnv = "SWITCHBOARD_ASKPASS_SOCK"

	askpassHostEnv = "SWITCHBOARD_HOST_ID"
	askpassTimeout = 5 * time.Minute
)

type askpassRequest struct {
	Prompt string `json:"prompt"`
}

type askpassResponse struct {
	Secret string `json:"secret,omitempty"`
	Error  string `json:"error,omitempty"`
}

type askpassServer struct {
	ctx      context.Context
	hostID   string
	secrets  *secretStore
	prompter Prompter
	dir      string
	sockPath string
	ln       net.Listener
	activity chan struct{}

	mu       sync.Mutex
	inFlight int
	closed   bool
}

func startAskpassServer(ctx context.Context, hostID string, secrets *secretStore, prompter Prompter) (*askpassServer, error) {
	dir, err := os.MkdirTemp("", "switchboard-askpass-")
	if err != nil {
		return nil, fmt.Errorf("askpass socket: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	sock := filepath.Join(dir, "sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("askpass listen: %w", err)
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		_ = ln.Close()
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if secrets == nil {
		secrets = newSecretStore()
	}
	s := &askpassServer{
		ctx:      ctx,
		hostID:   hostID,
		secrets:  secrets,
		prompter: prompter,
		dir:      dir,
		sockPath: sock,
		ln:       ln,
		activity: make(chan struct{}, 1),
	}
	go s.serve()
	go func() {
		<-ctx.Done()
		s.close()
	}()
	return s, nil
}

func (s *askpassServer) childEnv(askpassBin string) []string {
	return []string{
		"SSH_ASKPASS=" + askpassBin,
		"SSH_ASKPASS_REQUIRE=force",
		AskpassEnableEnv + "=1",
		AskpassSockEnv + "=" + s.sockPath,
		askpassHostEnv + "=" + s.hostID,
	}
}

func (s *askpassServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *askpassServer) handle(conn net.Conn) {
	defer conn.Close()
	s.begin()
	defer s.end()

	_ = conn.SetDeadline(time.Now().Add(askpassTimeout))

	var req askpassRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	promptReq := PromptRequest{
		HostID: s.hostID,
		Prompt: req.Prompt,
		Kind:   KindFromPrompt(req.Prompt),
	}
	secret, err := s.secrets.respond(s.ctx, promptReq, s.prompter)

	enc := json.NewEncoder(conn)
	if err != nil {
		_ = enc.Encode(askpassResponse{Error: err.Error()})
		return
	}
	_ = enc.Encode(askpassResponse{Secret: secret})
}

func (s *askpassServer) begin() {
	s.mu.Lock()
	s.inFlight++
	s.mu.Unlock()
	s.ping()
}

func (s *askpassServer) end() {
	s.mu.Lock()
	if s.inFlight > 0 {
		s.inFlight--
	}
	s.mu.Unlock()
	s.ping()
}

func (s *askpassServer) busy() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inFlight > 0
}

func (s *askpassServer) ping() {
	if s == nil || s.activity == nil {
		return
	}
	select {
	case s.activity <- struct{}{}:
	default:
	}
}

func (s *askpassServer) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	ln := s.ln
	dir := s.dir
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// IsAskpass reports whether this process was launched by OpenSSH as SSH_ASKPASS.
func IsAskpass() bool {
	return os.Getenv(AskpassEnableEnv) == "1"
}

// RunAskpass answers a single OpenSSH prompt over the manager's unix socket.
// It prints the secret to stdout and returns a process exit code.
func RunAskpass() int {
	sock := os.Getenv(AskpassSockEnv)
	if sock == "" {
		fmt.Fprintln(os.Stderr, "askpass: missing "+AskpassSockEnv)
		return 1
	}
	secret, err := queryAskpass(sock, askpassPrompt(os.Args))
	if err != nil {
		return 1
	}
	fmt.Print(secret)
	return 0
}

func askpassPrompt(args []string) string {
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "askpass" {
		rest = rest[1:]
	}
	return strings.Join(rest, " ")
}

func queryAskpass(sock, prompt string) (string, error) {
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(askpassTimeout))

	if err := json.NewEncoder(conn).Encode(askpassRequest{Prompt: prompt}); err != nil {
		return "", err
	}
	var resp askpassResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", fmt.Errorf("%s", resp.Error)
	}
	return resp.Secret, nil
}
