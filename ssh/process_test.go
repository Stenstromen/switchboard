package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeSSH(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh")
	body := "#!/bin/sh\n" + script + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartStopFakeSSH(t *testing.T) {
	bin := writeFakeSSH(t, `
printf '%s\n' "$@" > "$SWITCHBOARD_ARGS_FILE"
echo "running" >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("SWITCHBOARD_ARGS_FILE", argsFile)

	h := Host{
		HostName:              "server.example.com",
		User:                  "filip",
		Port:                  2222,
		ProxyJump:             "jump.example.com",
		StrictHostKeyChecking: "accept-new",
		HashKnownHosts:        true,
		NoSession:             true,
	}
	p, err := start(bin, h, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(argsFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake ssh did not write args file")
		}
		time.Sleep(20 * time.Millisecond)
	}

	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := withVerbose(SSHArgs(h, nil))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("child args = %q, want %q", got, want)
	}

	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	if p.Status() != StatusDisconnected {
		t.Fatalf("status = %q, want %q", p.Status(), StatusDisconnected)
	}
	if !strings.Contains(p.Logs(), "running") {
		t.Fatalf("logs = %q, want captured stderr", p.Logs())
	}
}

func TestStartImmediateFailure(t *testing.T) {
	bin := writeFakeSSH(t, `
echo "Permission denied" >&2
exit 255
`)
	p, err := start(bin, Host{HostName: "example.com"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Wait(); err == nil {
		t.Fatal("expected wait error")
	}
	if p.Status() != StatusError {
		t.Fatalf("status = %q, want %q", p.Status(), StatusError)
	}
	if !strings.Contains(p.Logs(), "Permission denied") {
		t.Fatalf("logs = %q", p.Logs())
	}
}
