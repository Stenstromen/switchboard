package ssh

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const DefaultSSHBinary = "/usr/bin/ssh"

// reservedExtra keys are emitted from dedicated Host fields, so ExtraOptions
// must not duplicate them.
var reservedExtra = map[string]bool{
	"ProxyJump":             true,
	"StrictHostKeyChecking": true,
	"HashKnownHosts":        true,
	"ServerAliveInterval":   true,
	"ServerAliveCountMax":   true,
	"Compression":           true,
	"BindAddress":           true,
	"AddressFamily":         true,
	"ForwardAgent":          true,
	"CertificateFile":       true,
	"LogLevel":              true,
	"ExitOnForwardFailure":  true,
}

// SSHArgs turns host configuration and tunnels into an OpenSSH argument
// slice. The destination is last. It never builds a shell command string.
func SSHArgs(h Host, tunnels []Tunnel) []string {
	args := []string{}
	if h.NoSession {
		args = append(args, "-N")
	}

	if h.Port > 0 {
		args = append(args, "-p", strconv.Itoa(h.Port))
	}
	if h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	if h.CertificateFile != "" {
		args = append(args, "-o", "CertificateFile="+h.CertificateFile)
	}
	if h.Compression {
		args = append(args, "-C")
	}
	if h.ForwardAgent {
		args = append(args, "-A")
	}
	if h.BindAddress != "" {
		args = append(args, "-b", h.BindAddress)
	}
	if h.ProxyJump != "" {
		args = append(args, "-o", "ProxyJump="+h.ProxyJump)
	}
	if h.StrictHostKeyChecking != "" {
		args = append(args, "-o", "StrictHostKeyChecking="+h.StrictHostKeyChecking)
	}
	if h.HashKnownHosts {
		args = append(args, "-o", "HashKnownHosts=yes")
	}
	if h.ServerAliveInterval > 0 {
		args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", h.ServerAliveInterval))
	}
	if h.ServerAliveCountMax > 0 {
		args = append(args, "-o", fmt.Sprintf("ServerAliveCountMax=%d", h.ServerAliveCountMax))
	}
	if af := normalizeAddressFamily(h.AddressFamily); af != "" {
		args = append(args, "-o", "AddressFamily="+af)
	}
	if h.LogLevel != "" {
		args = append(args, "-o", "LogLevel="+h.LogLevel)
	}
	if v := strings.TrimSpace(h.ExtraOptions["ExitOnForwardFailure"]); v != "" {
		args = append(args, "-o", "ExitOnForwardFailure="+v)
	} else if len(tunnels) > 0 {
		// Tunnel managers almost always want this; Advanced can override.
		args = append(args, "-o", "ExitOnForwardFailure=yes")
	}

	if len(h.ExtraOptions) > 0 {
		keys := make([]string, 0, len(h.ExtraOptions))
		for k, v := range h.ExtraOptions {
			if reservedExtra[k] || strings.TrimSpace(v) == "" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-o", k+"="+h.ExtraOptions[k])
		}
	}

	for _, t := range tunnels {
		flag, err := t.flag()
		if err != nil {
			continue
		}
		spec, err := t.spec()
		if err != nil {
			continue
		}
		args = append(args, flag, spec)
	}

	args = append(args, h.destination())
	return args
}

func normalizeAddressFamily(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "any":
		return ""
	case "inet", "ipv4", "4":
		return "inet"
	case "inet6", "ipv6", "6":
		return "inet6"
	default:
		return v
	}
}

// Validate reports configuration problems that would make an ssh invocation
// useless. SSHArgs itself stays a pure translator.
func Validate(h Host, tunnels []Tunnel) error {
	if strings.TrimSpace(h.HostName) == "" {
		return fmt.Errorf("HostName is required")
	}
	if h.Port < 0 {
		return fmt.Errorf("port must be non-negative")
	}
	for i, t := range tunnels {
		if _, err := t.flag(); err != nil {
			return fmt.Errorf("tunnel %d: %w", i, err)
		}
		if _, err := t.spec(); err != nil {
			return fmt.Errorf("tunnel %d: %w", i, err)
		}
	}
	return nil
}
