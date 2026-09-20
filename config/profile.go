package config

import (
	"strconv"

	"github.com/stenstromen/switchboard/ssh"
)

// Forward is a port-forward rule belonging to a tunnel profile.
type Forward struct {
	ID         string         `json:"id"`
	Type       ssh.TunnelType `json:"type"`
	LocalHost  string         `json:"localHost"`
	LocalPort  int            `json:"localPort"`
	RemoteHost string         `json:"remoteHost"`
	RemotePort int            `json:"remotePort"`
}

// Profile is the GUI unit of configuration: how to reach a host, what to
// forward, and a few UI preferences. Secrets are never stored here.
type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	HostName string `json:"hostName"`
	User     string `json:"user"`
	Port     int    `json:"port"`

	ProxyJump    string `json:"proxyJump"`
	IdentityFile string `json:"identityFile"`
	Certificate  string `json:"certificate"`

	StrictHostKeyChecking string `json:"strictHostKeyChecking"`
	HashKnownHosts        bool   `json:"hashKnownHosts"`

	ServerAliveInterval int `json:"serverAliveInterval"`
	ServerAliveCountMax int `json:"serverAliveCountMax"`

	// Connection tab
	LogLevel      string `json:"logLevel"`
	RetryAttempts int    `json:"retryAttempts"`
	BindAddress   string `json:"bindAddress"`
	AddressFamily string `json:"addressFamily"` // any | inet | inet6
	NoSession     bool   `json:"noSession"`     // -N (recommended)
	Compression   bool   `json:"compression"`
	ForwardAgent  bool   `json:"forwardAgent"`

	// Advanced OpenSSH -o Key=Value overrides (empty value = omit / default).
	Advanced map[string]string `json:"advanced,omitempty"`

	Forwards []Forward `json:"forwards"`

	NotifyOnStatusChange bool `json:"notifyOnStatusChange"`
	AutoConnect          bool `json:"autoConnect"`

	// Pinned tunnels stay at the top of the sidebar list.
	Pinned bool `json:"pinned,omitempty"`

	Tags []string `json:"tags,omitempty"`

	// DemoStatus / DemoErr are screenshot fixtures only (SWITCHBOARD_DEMO=1).
	// They are never passed to OpenSSH.
	DemoStatus string `json:"demoStatus,omitempty"`
	DemoErr    string `json:"demoErr,omitempty"`
}

// Document is the on-disk config file.
type Document struct {
	Version     int          `json:"version"`
	Profiles    []Profile    `json:"profiles"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

// ToHost maps a profile onto the SSH Host command-builder model.
func (p Profile) ToHost() ssh.Host {
	h := ssh.Host{
		Name:                  p.ID,
		HostName:              p.HostName,
		User:                  p.User,
		Port:                  p.Port,
		ProxyJump:             firstNonEmpty(p.ProxyJump, advanced(p, "ProxyJump")),
		IdentityFile:          p.IdentityFile,
		CertificateFile:       firstNonEmpty(p.Certificate, advanced(p, "CertificateFile")),
		StrictHostKeyChecking: firstNonEmpty(p.StrictHostKeyChecking, advanced(p, "StrictHostKeyChecking")),
		HashKnownHosts:        p.HashKnownHosts,
		ServerAliveInterval:   p.ServerAliveInterval,
		ServerAliveCountMax:   p.ServerAliveCountMax,
		NoSession:             p.NoSession,
		Compression:           p.Compression,
		BindAddress:           firstNonEmpty(p.BindAddress, advanced(p, "BindAddress")),
		AddressFamily:         p.AddressFamily,
		ForwardAgent:          p.ForwardAgent,
		LogLevel:              p.LogLevel,
		MaxRetries:            p.RetryAttempts,
		ExtraOptions:          copyMap(p.Advanced),
	}
	if h.ServerAliveInterval == 0 {
		if v := advancedInt(p, "ServerAliveInterval"); v > 0 {
			h.ServerAliveInterval = v
		}
	}
	if h.ServerAliveCountMax == 0 {
		if v := advancedInt(p, "ServerAliveCountMax"); v > 0 {
			h.ServerAliveCountMax = v
		}
	}
	return h
}

// ToTunnels maps profile forwards onto ssh.Tunnel values.
func (p Profile) ToTunnels() []ssh.Tunnel {
	out := make([]ssh.Tunnel, 0, len(p.Forwards))
	for _, f := range p.Forwards {
		out = append(out, ssh.Tunnel{
			Type:       f.Type,
			LocalHost:  f.LocalHost,
			LocalPort:  f.LocalPort,
			RemoteHost: f.RemoteHost,
			RemotePort: f.RemotePort,
		})
	}
	return out
}

// Destination is user@host display string.
func (p Profile) Destination() string {
	if p.User != "" {
		return p.User + "@" + p.HostName
	}
	return p.HostName
}

func NewProfile() Profile {
	return Profile{
		Name:                 "Unnamed",
		Port:                 22,
		NoSession:            true,
		LogLevel:             "INFO",
		RetryAttempts:        999,
		AddressFamily:        "any",
		NotifyOnStatusChange: true,
		AutoConnect:          false,
		ServerAliveInterval:  15,
		ServerAliveCountMax:  3,
		Advanced:             map[string]string{},
		Forwards:             []Forward{},
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func advanced(p Profile, key string) string {
	if p.Advanced == nil {
		return ""
	}
	return p.Advanced[key]
}

func advancedInt(p Profile, key string) int {
	v := advanced(p, key)
	if v == "" || v == "none" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func copyMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
