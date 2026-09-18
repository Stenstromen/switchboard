package config

import (
	"os"
	"path/filepath"
)

// ShowAs controls whether Switchboard appears in the Dock, menu bar, or both.
type ShowAs string

const (
	ShowAsDock    ShowAs = "dock"
	ShowAsMenuBar ShowAs = "menubar"
	ShowAsBoth    ShowAs = "both"
)

// EnvVar is an optional environment variable injected into ssh child processes.
type EnvVar struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// Preferences are app-wide settings (not per-tunnel).
type Preferences struct {
	ShowAs      ShowAs   `json:"showAs"`
	OpenAtLogin bool     `json:"openAtLogin"`
	AuthAgent   string   `json:"authAgent"`
	ConfigFile  string   `json:"configFile"`
	EnvVars     []EnvVar `json:"envVars,omitempty"`
}

// DefaultPreferences is used for new installs and missing fields.
func DefaultPreferences() Preferences {
	return Preferences{
		ShowAs:      ShowAsBoth,
		OpenAtLogin: false,
		EnvVars:     []EnvVar{},
	}
}

func (p Preferences) Normalize() Preferences {
	switch p.ShowAs {
	case ShowAsDock, ShowAsMenuBar, ShowAsBoth:
	default:
		p.ShowAs = ShowAsBoth
	}
	if p.EnvVars == nil {
		p.EnvVars = []EnvVar{}
	}
	return p
}

func (s ShowAs) Normalize() ShowAs {
	switch s {
	case ShowAsDock, ShowAsMenuBar, ShowAsBoth:
		return s
	default:
		return ShowAsBoth
	}
}

// DefaultAuthAgent is the current process SSH_AUTH_SOCK (may be empty).
func DefaultAuthAgent() string {
	return os.Getenv("SSH_AUTH_SOCK")
}

// DefaultConfigFile is ~/.ssh/config.
func DefaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

// DefaultKnownHostsFile is ~/.ssh/known_hosts.
func DefaultKnownHostsFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// EffectiveAuthAgent returns the override or the process default.
func (p Preferences) EffectiveAuthAgent() string {
	if p.AuthAgent != "" {
		return p.AuthAgent
	}
	return DefaultAuthAgent()
}

// EffectiveConfigFile returns the override or ~/.ssh/config.
func (p Preferences) EffectiveConfigFile() string {
	if p.ConfigFile != "" {
		return p.ConfigFile
	}
	return DefaultConfigFile()
}

// EnabledEnvPairs returns NAME=value for enabled entries with a non-empty name.
func (p Preferences) EnabledEnvPairs() []string {
	out := make([]string, 0, len(p.EnvVars))
	for _, e := range p.EnvVars {
		if !e.Enabled || e.Name == "" {
			continue
		}
		out = append(out, e.Name+"="+e.Value)
	}
	return out
}

// SSHRuntimeChanged reports whether auth agent, config file, or env vars differ.
func SSHRuntimeChanged(a, b Preferences) bool {
	if a.AuthAgent != b.AuthAgent || a.ConfigFile != b.ConfigFile {
		return true
	}
	if len(a.EnvVars) != len(b.EnvVars) {
		return true
	}
	for i := range a.EnvVars {
		if a.EnvVars[i] != b.EnvVars[i] {
			return true
		}
	}
	return false
}
