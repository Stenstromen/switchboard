package ssh

import (
	"fmt"
	"strconv"
)

// TunnelType is an OpenSSH forwarding mode.
type TunnelType string

const (
	TunnelLocal          TunnelType = "local"
	TunnelRemote         TunnelType = "remote"
	TunnelDynamic        TunnelType = "dynamic"
	TunnelReverseDynamic TunnelType = "reverse-dynamic"
)

// Tunnel describes what an SSH session should forward. It is independent of
// how the server is reached (see Host).
type Tunnel struct {
	Type       TunnelType
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
}

func (t Tunnel) flag() (string, error) {
	switch t.Type {
	case TunnelLocal:
		return "-L", nil
	case TunnelRemote, TunnelReverseDynamic:
		return "-R", nil
	case TunnelDynamic:
		return "-D", nil
	default:
		if t.Type == "" {
			return "", fmt.Errorf("tunnel type is required")
		}
		return "", fmt.Errorf("unknown tunnel type %q", t.Type)
	}
}

func (t Tunnel) spec() (string, error) {
	if t.LocalPort <= 0 {
		return "", fmt.Errorf("local port must be positive")
	}

	switch t.Type {
	case TunnelDynamic, TunnelReverseDynamic:
		if t.LocalHost != "" {
			return fmt.Sprintf("%s:%d", t.LocalHost, t.LocalPort), nil
		}
		return strconv.Itoa(t.LocalPort), nil
	case TunnelLocal, TunnelRemote:
		if t.RemoteHost == "" {
			return "", fmt.Errorf("remote host is required")
		}
		if t.RemotePort <= 0 {
			return "", fmt.Errorf("remote port must be positive")
		}
		local := strconv.Itoa(t.LocalPort)
		if t.LocalHost != "" {
			local = fmt.Sprintf("%s:%d", t.LocalHost, t.LocalPort)
		}
		return fmt.Sprintf("%s:%s:%d", local, t.RemoteHost, t.RemotePort), nil
	default:
		return "", fmt.Errorf("unknown tunnel type %q", t.Type)
	}
}
