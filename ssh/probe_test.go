package ssh

import (
	"net"
	"testing"
	"time"
)

func TestProbeableLocalAddrs(t *testing.T) {
	addrs := probeableLocalAddrs([]Tunnel{
		{Type: TunnelLocal, LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 80},
		{Type: TunnelDynamic, LocalHost: "127.0.0.1", LocalPort: 1080},
		{Type: TunnelRemote, LocalPort: 9090, RemoteHost: "localhost", RemotePort: 9090},
		{Type: TunnelLocal, LocalHost: "0.0.0.0", LocalPort: 3000, RemoteHost: "h", RemotePort: 1},
	})
	if len(addrs) != 3 {
		t.Fatalf("got %d addrs, want 3: %+v", len(addrs), addrs)
	}
	if addrs[0].Host != "127.0.0.1" || addrs[0].Port != 8080 {
		t.Fatalf("first = %+v", addrs[0])
	}
	if addrs[2].Host != "127.0.0.1" || addrs[2].Port != 3000 {
		t.Fatalf("wildcard should map to loopback, got %+v", addrs[2])
	}
}

func TestSessionLooksStaleRemoteOnly(t *testing.T) {
	tunnels := []Tunnel{
		{Type: TunnelRemote, LocalPort: 9090, RemoteHost: "localhost", RemotePort: 9090},
	}
	if !sessionLooksStale(1, tunnels) {
		t.Fatal("remote-only should be treated as stale (unprobeable)")
	}
}

func TestProbeLocalForwards(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	tunnels := []Tunnel{{Type: TunnelDynamic, LocalPort: port}}
	if !probeLocalForwards(tunnels) {
		ln.Close()
		t.Fatal("expected listening port to probe OK")
	}
	_ = ln.Close()
	time.Sleep(20 * time.Millisecond)
	if probeLocalForwards(tunnels) {
		t.Fatalf("closed port %d should fail probe", port)
	}
}

func TestEffectiveServerAliveDefaults(t *testing.T) {
	interval, count := effectiveServerAlive(Host{})
	if interval != defaultServerAliveInterval || count != defaultServerAliveCountMax {
		t.Fatalf("got %d/%d", interval, count)
	}
	interval, count = effectiveServerAlive(Host{ServerAliveInterval: 30, ServerAliveCountMax: 5})
	if interval != 30 || count != 5 {
		t.Fatalf("got %d/%d", interval, count)
	}
}
