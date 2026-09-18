package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KnownHostEntry is one non-comment line from a known_hosts file.
type KnownHostEntry struct {
	Index   int    `json:"index"`
	Line    string `json:"line"`
	Hosts   string `json:"hosts"`
	KeyType string `json:"keyType"`
	KeyData string `json:"keyData"`
}

// KnownHostsPath is the default known_hosts location.
func KnownHostsPath() string {
	return DefaultKnownHostsFile()
}

// ListKnownHosts reads and parses the known_hosts file.
func ListKnownHosts(path string) ([]KnownHostEntry, error) {
	if path == "" {
		path = KnownHostsPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []KnownHostEntry{}, nil
		}
		return nil, err
	}
	return parseKnownHosts(string(raw)), nil
}

func parseKnownHosts(content string) []KnownHostEntry {
	lines := strings.Split(content, "\n")
	out := make([]KnownHostEntry, 0)
	idx := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entry := KnownHostEntry{Index: idx, Line: trimmed}
		idx++
		fields := strings.Fields(trimmed)
		if len(fields) >= 3 {
			entry.Hosts = fields[0]
			entry.KeyType = fields[1]
			entry.KeyData = fields[2]
		} else if len(fields) == 2 {
			entry.Hosts = fields[0]
			entry.KeyType = fields[1]
		} else if len(fields) == 1 {
			entry.Hosts = fields[0]
		}
		out = append(out, entry)
	}
	return out
}

// AddKnownHostLine appends a known_hosts entry (full line).
func AddKnownHostLine(path, line string) error {
	if path == "" {
		path = KnownHostsPath()
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fmt.Errorf("empty known host entry")
	}
	if strings.ContainsAny(line, "\n\r") {
		return fmt.Errorf("known host entry must be a single line")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}

// RemoveKnownHostAt removes the entry at the given list index (from ListKnownHosts).
func RemoveKnownHostAt(path string, index int) error {
	if path == "" {
		path = KnownHostsPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	keep := make([]string, 0, len(lines))
	entryIdx := 0
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			keep = append(keep, line)
			continue
		}
		if entryIdx == index {
			removed = true
			entryIdx++
			continue
		}
		keep = append(keep, line)
		entryIdx++
	}
	if !removed {
		return fmt.Errorf("known host index %d not found", index)
	}
	body := strings.Join(keep, "\n")
	if !strings.HasSuffix(body, "\n") && body != "" {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body), 0o600)
}
