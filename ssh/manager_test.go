package ssh

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerConnectDisconnect(t *testing.T) {
	bin := writeFakeSSH(t, `
echo "Authenticated to fake ([127.0.0.1]:22)." >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)
	mgr := NewManager(
		WithSSHBinary(bin),
		WithRuntimeDir(t.TempDir()),
		WithStableAfter(40*time.Millisecond),
		WithBackoff(Backoff{Initial: 20 * time.Millisecond, Max: 50 * time.Millisecond}),
	)
	defer mgr.Close()

	h := Host{Name: "lab", HostName: "server.example.com", User: "filip"}
	if err := mgr.AddHost(h); err != nil {
		t.Fatal(err)
	}
	if err := mgr.AddTunnel("lab", Tunnel{Type: TunnelLocal, LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 80}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}

	waitStatus(t, mgr, "lab", StatusConnected)
	if err := mgr.Disconnect("lab"); err != nil {
		t.Fatal(err)
	}
	if got := mgr.Status("lab").Status; got != StatusDisconnected {
		t.Fatalf("status = %q, want %q", got, StatusDisconnected)
	}
}

func TestManagerReconnectsAfterExit(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "count")
	t.Setenv("SWITCHBOARD_COUNT", countFile)
	bin := writeFakeSSH(t, `
n=0
if [ -f "$SWITCHBOARD_COUNT" ]; then n=$(cat "$SWITCHBOARD_COUNT"); fi
n=$((n+1))
echo "$n" > "$SWITCHBOARD_COUNT"
if [ "$n" -lt 3 ]; then
  echo "fail $n" >&2
  exit 255
fi
echo "Authenticated to fake ([127.0.0.1]:22)." >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)
	mgr := NewManager(
		WithSSHBinary(bin),
		WithRuntimeDir(t.TempDir()),
		WithStableAfter(40*time.Millisecond),
		WithBackoff(Backoff{Initial: 20 * time.Millisecond, Max: 40 * time.Millisecond}),
	)
	defer mgr.Close()

	if err := mgr.AddHost(Host{Name: "lab", HostName: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, mgr, "lab", StatusConnected)
	if err := mgr.Disconnect("lab"); err != nil {
		t.Fatal(err)
	}
}

func TestManagerAuthFailureDoesNotReconnect(t *testing.T) {
	countFile := filepath.Join(t.TempDir(), "count")
	t.Setenv("SWITCHBOARD_COUNT", countFile)
	bin := writeFakeSSH(t, `
n=0
if [ -f "$SWITCHBOARD_COUNT" ]; then n=$(cat "$SWITCHBOARD_COUNT"); fi
n=$((n+1))
echo "$n" > "$SWITCHBOARD_COUNT"
echo "Permission denied (publickey,password)." >&2
exit 255
`)
	mgr := NewManager(
		WithSSHBinary(bin),
		WithRuntimeDir(t.TempDir()),
		WithStableAfter(40*time.Millisecond),
		WithBackoff(Backoff{Initial: 20 * time.Millisecond, Max: 40 * time.Millisecond}),
	)
	defer mgr.Close()

	if err := mgr.AddHost(Host{Name: "lab", HostName: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, mgr, "lab", StatusError)
	time.Sleep(150 * time.Millisecond)
	raw, err := os.ReadFile(countFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "1\n" {
		t.Fatalf("ssh starts = %q, want 1 (no reconnect on auth failure)", raw)
	}
}

func TestManagerUnknownHost(t *testing.T) {
	mgr := NewManager()
	if err := mgr.Connect("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func waitStatus(t *testing.T, mgr *Manager, id string, want Status) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last Snapshot
	for time.Now().Before(deadline) {
		last = mgr.Status(id)
		if last.Status == want {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("status = %+v, want %q", last, want)
}
