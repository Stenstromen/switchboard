package ssh

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	localProbeTimeout = 400 * time.Millisecond
	// Default keepalives for tunnel sessions. OpenSSH defaults (0) never
	// detect a half-dead link after sleep/wake on their own.
	defaultServerAliveInterval = 15
	defaultServerAliveCountMax = 3
)

// localListenAddr is a TCP address ssh should be accepting on this machine.
type localListenAddr struct {
	Host string
	Port int
}

// probeableLocalAddrs returns listen targets for local and dynamic forwards.
// Remote-only tunnels have nothing to probe on the client.
func probeableLocalAddrs(tunnels []Tunnel) []localListenAddr {
	out := make([]localListenAddr, 0, len(tunnels))
	for _, t := range tunnels {
		switch t.Type {
		case TunnelLocal, TunnelDynamic:
			if t.LocalPort <= 0 {
				continue
			}
			host := strings.TrimSpace(t.LocalHost)
			if host == "" || host == "*" || host == "0.0.0.0" || host == "::" {
				host = "127.0.0.1"
			}
			out = append(out, localListenAddr{Host: host, Port: t.LocalPort})
		}
	}
	return out
}

// probeLocalForwards dials each local/dynamic listen address. Returns false if
// any expected listener refuses or times out (typical after sleep left a zombie
// ssh that dropped its binds, or the process is wedged).
func probeLocalForwards(tunnels []Tunnel) bool {
	addrs := probeableLocalAddrs(tunnels)
	if len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if !dialLocal(a) {
			return false
		}
	}
	return true
}

func dialLocal(a localListenAddr) bool {
	addr := net.JoinHostPort(a.Host, strconv.Itoa(a.Port))
	conn, err := net.DialTimeout("tcp", addr, localProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// hasEstablishedTCP reports whether pid still has at least one ESTABLISHED TCP
// socket. After sleep/wake, a zombie ssh often loses its remote transport while
// the process (and sometimes local listens) survive.
func hasEstablishedTCP(pid int) bool {
	if pid <= 0 {
		return false
	}
	// lsof is present on macOS; -sTCP:ESTABLISHED filters to live transports.
	cmd := exec.Command("/usr/sbin/lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:ESTABLISHED")
	out, err := cmd.Output()
	if err != nil {
		// Exit 1 = no matches; treat as not established.
		return false
	}
	// Header line "COMMAND PID ..." plus at least one data row.
	lines := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines++
	}
	return lines >= 2
}

// sessionLooksStale decides whether a "connected" session should be bounced
// after sleep/wake. Remote-only tunnels cannot be probed locally and are
// treated as stale. Local/dynamic tunnels must accept dials and the ssh child
// must still show an ESTABLISHED TCP socket.
func sessionLooksStale(pid int, tunnels []Tunnel) bool {
	addrs := probeableLocalAddrs(tunnels)
	if len(addrs) == 0 {
		return true
	}
	if !probeLocalForwards(tunnels) {
		return true
	}
	if !hasEstablishedTCP(pid) {
		return true
	}
	return false
}

// effectiveServerAlive returns interval/count, applying tunnel-manager defaults
// when the host left them unset.
func effectiveServerAlive(h Host) (interval, count int) {
	interval = h.ServerAliveInterval
	count = h.ServerAliveCountMax
	if interval <= 0 {
		interval = defaultServerAliveInterval
	}
	if count <= 0 {
		count = defaultServerAliveCountMax
	}
	return interval, count
}
