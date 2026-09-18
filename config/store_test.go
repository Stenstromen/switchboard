package config

import (
	"testing"

	"github.com/stenstromen/switchboard/ssh"
)

func TestProfileToHostAndTunnels(t *testing.T) {
	p := NewProfile()
	p.ID = "lab"
	p.Name = "MailTun"
	p.HostName = "example.com"
	p.User = "filip"
	p.Port = 22
	p.ProxyJump = "jump"
	p.Forwards = []Forward{{
		Type: ssh.TunnelLocal, LocalHost: "127.0.0.1", LocalPort: 1143,
		RemoteHost: "127.0.0.1", RemotePort: 143,
	}}

	h := p.ToHost()
	if h.Name != "lab" || h.HostName != "example.com" || !h.NoSession {
		t.Fatalf("host = %+v", h)
	}
	if h.MaxRetries != 999 {
		t.Fatalf("MaxRetries = %d, want 999 from RetryAttempts default", h.MaxRetries)
	}
	tunnels := p.ToTunnels()
	if len(tunnels) != 1 || tunnels[0].LocalPort != 1143 {
		t.Fatalf("tunnels = %+v", tunnels)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := t.TempDir() + "/tunnels.json"
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := NewProfile()
	p.ID = "a"
	p.Name = "Test"
	p.HostName = "h.example"
	if _, err := store.Upsert(p); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get("a")
	if err != nil || !ok {
		t.Fatalf("get: %v ok=%v", err, ok)
	}
	if got.Name != "Test" || got.HostName != "h.example" {
		t.Fatalf("got %+v", got)
	}
}

func TestPreferencesRoundTrip(t *testing.T) {
	path := t.TempDir() + "/tunnels.json"
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	prefs := Preferences{ShowAs: ShowAsMenuBar, OpenAtLogin: true}
	if err := store.SetPreferences(prefs); err != nil {
		t.Fatal(err)
	}
	got, err := store.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if got.ShowAs != ShowAsMenuBar || !got.OpenAtLogin {
		t.Fatalf("got %+v", got)
	}
}
