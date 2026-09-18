package ssh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKindFromPrompt(t *testing.T) {
	if KindFromPrompt("filip@host's password: ") != SecretPassword {
		t.Fatal("expected password")
	}
	if KindFromPrompt("Enter passphrase for key '/tmp/id': ") != SecretPassphrase {
		t.Fatal("expected passphrase")
	}
}

func TestAskpassServerCachedPassword(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := newSecretStore()
	store.setPassword("s3cret")
	srv, err := startAskpassServer(ctx, "lab", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	got, err := queryAskpass(srv.sockPath, "filip@host's password: ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "s3cret" {
		t.Fatalf("got %q", got)
	}
}

func TestAskpassServerPrompter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prompter := func(_ context.Context, req PromptRequest) (string, error) {
		if req.HostID != "lab" {
			t.Errorf("host id = %q", req.HostID)
		}
		if req.Kind != SecretPassphrase {
			t.Errorf("kind = %q", req.Kind)
		}
		return "phrase", nil
	}
	srv, err := startAskpassServer(ctx, "lab", newSecretStore(), prompter)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.close()

	got, err := queryAskpass(srv.sockPath, "Enter passphrase for key 'id': ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "phrase" {
		t.Fatalf("got %q", got)
	}
}

func TestAskpassRejectedSecretFallsThrough(t *testing.T) {
	store := newSecretStore()
	store.setPassword("wrong")
	req := PromptRequest{Prompt: "password: ", Kind: SecretPassword}

	first, err := store.respond(context.Background(), req, nil)
	if err != nil || first != "wrong" {
		t.Fatalf("first = %q, %v", first, err)
	}

	called := false
	second, err := store.respond(context.Background(), req, func(context.Context, PromptRequest) (string, error) {
		called = true
		return "right", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected prompter after rejected cache")
	}
	if second != "right" {
		t.Fatalf("second = %q", second)
	}
}

func TestAskpassCancelStopsFurtherPrompts(t *testing.T) {
	store := newSecretStore()
	_, err := store.respond(context.Background(), PromptRequest{Prompt: "password: "}, func(context.Context, PromptRequest) (string, error) {
		return "", ErrPromptCancelled
	})
	if !errors.Is(err, ErrPromptCancelled) {
		t.Fatalf("err = %v", err)
	}
	_, err = store.respond(context.Background(), PromptRequest{Prompt: "password: "}, func(context.Context, PromptRequest) (string, error) {
		t.Fatal("prompter should not be called after cancel")
		return "", nil
	})
	if !errors.Is(err, ErrPromptCancelled) {
		t.Fatalf("err = %v", err)
	}
}

func TestManagerAskpassPassword(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(t.TempDir(), "secret")
	t.Setenv("SWITCHBOARD_SECRET_FILE", secretFile)

	bin := writeFakeSSH(t, `
secret=$("$SSH_ASKPASS" "filip@host's password: ") || exit 255
printf '%s' "$secret" > "$SWITCHBOARD_SECRET_FILE"
echo "Authenticated to fake ([127.0.0.1]:22)." >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)
	mgr := NewManager(
		WithSSHBinary(bin),
		WithAskpassBinary(exe),
		WithStableAfter(40*time.Millisecond),
		WithBackoff(Backoff{Initial: 20 * time.Millisecond, Max: 50 * time.Millisecond}),
	)
	defer mgr.Close()

	if err := mgr.AddHost(Host{Name: "lab", HostName: "example.com", User: "filip"}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetPassword("lab", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, mgr, "lab", StatusConnected)

	raw, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "s3cret" {
		t.Fatalf("askpass delivered %q", raw)
	}
}

func TestManagerAskpassPrompter(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(t.TempDir(), "secret")
	t.Setenv("SWITCHBOARD_SECRET_FILE", secretFile)

	bin := writeFakeSSH(t, `
secret=$("$SSH_ASKPASS" "Password: ") || exit 255
printf '%s' "$secret" > "$SWITCHBOARD_SECRET_FILE"
echo "Authenticated to fake ([127.0.0.1]:22)." >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)
	mgr := NewManager(
		WithSSHBinary(bin),
		WithAskpassBinary(exe),
		WithStableAfter(40*time.Millisecond),
		WithPrompter(func(_ context.Context, req PromptRequest) (string, error) {
			if req.Kind != SecretPassword {
				t.Errorf("kind = %q", req.Kind)
			}
			return "from-ui", nil
		}),
	)
	defer mgr.Close()

	if err := mgr.AddHost(Host{Name: "lab", HostName: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, mgr, "lab", StatusConnected)

	raw, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "from-ui" {
		t.Fatalf("askpass delivered %q", raw)
	}
}
