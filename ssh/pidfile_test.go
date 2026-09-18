package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLooksLikeOurSSH(t *testing.T) {
	host := Host{HostName: "server.example.com"}
	tunnels := []Tunnel{{Type: TunnelLocal, LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 80}}

	cases := []struct {
		cmd  string
		want bool
	}{
		{"/usr/bin/ssh -N -L 8080:127.0.0.1:80 user@server.example.com", true},
		{"/tmp/fake/ssh -N -v", true}, // soft match via basename
		{"/bin/sh /tmp/fake/ssh", true}, // shebang fake
		{"/usr/bin/curl https://example.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeOurSSH(tc.cmd, host, tunnels); got != tc.want {
			t.Fatalf("looksLikeOurSSH(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestIsBindConflict(t *testing.T) {
	if !isBindConflict("bind [127.0.0.1]:8080: Address already in use") {
		t.Fatal("expected bind conflict")
	}
	if isBindConflict("Authenticated to host") {
		t.Fatal("did not expect bind conflict")
	}
}

func TestPidfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	host := Host{HostName: "example.com"}
	if err := writePID(dir, "lab", 4242, host); err != nil {
		t.Fatal(err)
	}
	rec, err := readPID(dir, "lab")
	if err != nil {
		t.Fatal(err)
	}
	if rec.PID != 4242 || rec.HostID != "lab" || rec.HostName != "example.com" {
		t.Fatalf("record = %+v", rec)
	}
	clearPID(dir, "lab")
	if _, err := readPID(dir, "lab"); !os.IsNotExist(err) {
		t.Fatalf("expected missing pidfile, got %v", err)
	}
}

func TestAdoptAndStop(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	defer killProcessGroup(pid)

	p, err := Adopt(pid)
	if err != nil {
		t.Fatal(err)
	}
	if p.PID() != pid {
		t.Fatalf("pid = %d, want %d", p.PID(), pid)
	}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatal("process still alive after Stop")
	}
}

func TestManagerAdoptsOrphan(t *testing.T) {
	runtime := t.TempDir()
	bin := writeFakeSSH(t, `
echo "Authenticated to fake ([127.0.0.1]:22)." >&2
trap 'exit 0' TERM INT
while true; do sleep 0.05; done
`)

	host := Host{Name: "lab", HostName: "server.example.com", User: "filip"}
	orphan := exec.Command(bin)
	orphan.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := orphan.Start(); err != nil {
		t.Fatal(err)
	}
	defer killProcessGroup(orphan.Process.Pid)
	if err := writePID(runtime, "lab", orphan.Process.Pid, host); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(
		WithSSHBinary(bin),
		WithRuntimeDir(runtime),
		WithStableAfter(40*time.Millisecond),
		WithBackoff(Backoff{Initial: 20 * time.Millisecond, Max: 50 * time.Millisecond}),
	)
	defer mgr.Close()

	if err := mgr.AddHost(host); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Connect("lab"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, mgr, "lab", StatusConnected)

	snap := mgr.Status("lab")
	if snap.PID != orphan.Process.Pid {
		t.Fatalf("adopted pid = %d, want orphan %d (started a duplicate?)", snap.PID, orphan.Process.Pid)
	}

	// Ensure we did not spawn a second child (count files in runtime is 1 pidfile).
	entries, _ := filepath.Glob(filepath.Join(runtime, "*.pid"))
	if len(entries) != 1 {
		t.Fatalf("pidfiles = %v, want 1", entries)
	}

	if err := mgr.Disconnect("lab"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for processAlive(orphan.Process.Pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if processAlive(orphan.Process.Pid) {
		t.Fatal("orphan still alive after Disconnect")
	}
	if _, err := os.Stat(pidPath(runtime, "lab")); !os.IsNotExist(err) {
		t.Fatalf("pidfile should be cleared, err=%v", err)
	}
}
