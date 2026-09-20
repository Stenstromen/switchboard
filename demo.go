package main

import (
	"os"
	"strings"
	"sync"

	"github.com/stenstromen/switchboard/config"
	"github.com/stenstromen/switchboard/ssh"
)

// demoMode enables screenshot fixtures: fake connection status, no real ssh.
func demoMode() bool {
	v := strings.TrimSpace(os.Getenv("SWITCHBOARD_DEMO"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

type demoOverlay struct {
	mu   sync.Mutex
	byID map[string]ssh.Snapshot
}

func newDemoOverlay() *demoOverlay {
	return &demoOverlay{byID: make(map[string]ssh.Snapshot)}
}

func (d *demoOverlay) seed(profiles []config.Profile) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range profiles {
		if p.ID == "" {
			continue
		}
		d.byID[p.ID] = snapshotFromDemoFields(p)
	}
}

func snapshotFromDemoFields(p config.Profile) ssh.Snapshot {
	st := ssh.Status(strings.TrimSpace(p.DemoStatus))
	switch st {
	case ssh.StatusConnected, ssh.StatusConnecting, ssh.StatusError, ssh.StatusDisconnected:
	default:
		st = ssh.StatusDisconnected
	}
	return ssh.Snapshot{Status: st, Err: strings.TrimSpace(p.DemoErr)}
}

func (d *demoOverlay) get(id string) (ssh.Snapshot, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	snap, ok := d.byID[id]
	return snap, ok
}

func (d *demoOverlay) set(id string, snap ssh.Snapshot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.byID[id] = snap
}
