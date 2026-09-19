package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Option configures a Manager.
type Option func(*Manager)

// WithSSHBinary overrides DefaultSSHBinary. Tests use this to inject a fake ssh.
func WithSSHBinary(path string) Option {
	return func(m *Manager) {
		m.sshBinary = path
	}
}

// WithAskpassBinary overrides the SSH_ASKPASS helper (defaults to this executable).
func WithAskpassBinary(path string) Option {
	return func(m *Manager) {
		m.askpassBin = path
	}
}

// WithBackoff sets reconnect backoff.
func WithBackoff(b Backoff) Option {
	return func(m *Manager) {
		m.backoff = b
	}
}

// WithStableAfter is how long ssh must stay alive after authentication
// before the session is Connected.
func WithStableAfter(d time.Duration) Option {
	return func(m *Manager) {
		m.stableAfter = d
	}
}

// WithOnStatus is called whenever a host's status changes.
func WithOnStatus(fn func(hostID string, snap Snapshot)) Option {
	return func(m *Manager) {
		m.onStatus = fn
	}
}

// WithPrompter is called when OpenSSH needs a password or key passphrase
// that is not already cached.
func WithPrompter(p Prompter) Option {
	return func(m *Manager) {
		m.prompter = p
	}
}

// WithRuntimeDir sets where per-tunnel pidfiles are stored for orphan reclaim.
func WithRuntimeDir(dir string) Option {
	return func(m *Manager) {
		m.runtimeDir = dir
	}
}

// Manager is the GUI-facing SSH subsystem: hosts and tunnels in, Connect(hostID)
// out. It never exposes exec.Cmd.
type Manager struct {
	sshBinary   string
	askpassBin  string
	backoff     Backoff
	stableAfter time.Duration
	runtimeDir  string
	onStatus    func(hostID string, snap Snapshot)
	prompter    Prompter

	mu       sync.Mutex
	hosts    map[string]Host
	tunnels  map[string][]Tunnel
	sessions map[string]*managed
	secrets  map[string]*secretStore
	global   GlobalOptions
}

// GlobalOptions are app-wide SSH settings applied to every session.
type GlobalOptions struct {
	AuthSock   string
	ConfigFile string
	Env        []string // NAME=value
}

type managed struct {
	watchdog *watchdog
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewManager returns an idle process manager around OpenSSH.
func NewManager(opts ...Option) *Manager {
	runtimeDir, _ := DefaultRuntimeDir()
	m := &Manager{
		sshBinary:   DefaultSSHBinary,
		backoff:     Backoff{Initial: time.Second, Max: time.Minute},
		stableAfter: 300 * time.Millisecond,
		runtimeDir:  runtimeDir,
		hosts:       make(map[string]Host),
		tunnels:     make(map[string][]Tunnel),
		sessions:    make(map[string]*managed),
		secrets:     make(map[string]*secretStore),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// AddHost registers or replaces host configuration. Connecting sessions are
// not restarted automatically.
func (m *Manager) AddHost(h Host) error {
	id := h.id()
	if id == "" {
		return fmt.Errorf("host Name or HostName is required")
	}
	if h.HostName == "" {
		return fmt.Errorf("HostName is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[id] = h
	return nil
}

// AddTunnel appends a forwarding rule to a registered host.
func (m *Manager) AddTunnel(hostID string, t Tunnel) error {
	if _, err := t.flag(); err != nil {
		return err
	}
	if _, err := t.spec(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hosts[hostID]; !ok {
		return fmt.Errorf("host %q not found", hostID)
	}
	m.tunnels[hostID] = append(m.tunnels[hostID], t)
	return nil
}

// SetTunnels replaces all forwarding rules for a registered host.
func (m *Manager) SetTunnels(hostID string, tunnels []Tunnel) error {
	for i, t := range tunnels {
		if _, err := t.flag(); err != nil {
			return fmt.Errorf("tunnel %d: %w", i, err)
		}
		if _, err := t.spec(); err != nil {
			return fmt.Errorf("tunnel %d: %w", i, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hosts[hostID]; !ok {
		return fmt.Errorf("host %q not found", hostID)
	}
	m.tunnels[hostID] = append([]Tunnel(nil), tunnels...)
	return nil
}

// SetPassword caches a login password for hostID. OpenSSH receives it through
// SSH_ASKPASS; it is never written into the argument slice or Host config.
func (m *Manager) SetPassword(hostID, password string) error {
	return m.setSecret(hostID, SecretPassword, password)
}

// SetPassphrase caches a private-key passphrase for hostID.
func (m *Manager) SetPassphrase(hostID, passphrase string) error {
	return m.setSecret(hostID, SecretPassphrase, passphrase)
}

// ClearSecrets drops cached passwords and passphrases for hostID.
func (m *Manager) ClearSecrets(hostID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.secrets[hostID]; s != nil {
		s.clear()
	}
}

func (m *Manager) setSecret(hostID string, kind SecretKind, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hosts[hostID]; !ok {
		return fmt.Errorf("host %q not found", hostID)
	}
	store := m.secretStoreLocked(hostID)
	if kind == SecretPassphrase {
		store.setPassphrase(secret)
		return nil
	}
	store.setPassword(secret)
	return nil
}

func (m *Manager) secretStoreLocked(hostID string) *secretStore {
	if m.secrets[hostID] == nil {
		m.secrets[hostID] = newSecretStore()
	}
	return m.secrets[hostID]
}

// SetGlobalOptions updates app-wide SSH env/config used by new and reconnecting sessions.
func (m *Manager) SetGlobalOptions(o GlobalOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.global = o
}

func (m *Manager) globalOptions() GlobalOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := GlobalOptions{
		AuthSock:   m.global.AuthSock,
		ConfigFile: m.global.ConfigFile,
		Env:        append([]string(nil), m.global.Env...),
	}
	return out
}

// Connect starts ssh for hostID and keeps it alive until Disconnect.
func (m *Manager) Connect(hostID string) error {
	m.mu.Lock()
	host, ok := m.hosts[hostID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("host %q not found", hostID)
	}
	tunnels := append([]Tunnel(nil), m.tunnels[hostID]...)
	if err := Validate(host, tunnels); err != nil {
		m.mu.Unlock()
		return err
	}
	if sess, ok := m.sessions[hostID]; ok {
		snap := sess.watchdog.snapshot()
		if snap.Status == StatusConnecting || snap.Status == StatusConnected {
			m.mu.Unlock()
			return fmt.Errorf("host %q is already %s", hostID, snap.Status)
		}
		delete(m.sessions, hostID)
		m.mu.Unlock()
		sess.cancel()
		<-sess.done
		m.mu.Lock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	var onStatus func(Snapshot)
	if m.onStatus != nil {
		cb := m.onStatus
		onStatus = func(snap Snapshot) {
			cb(hostID, snap)
		}
	}
	w := newWatchdog(watchdogConfig{
		host:        host,
		hostID:      hostID,
		tunnels:     tunnels,
		sshBinary:   m.sshBinary,
		askpassBin:  m.askpassBin,
		backoff:     m.backoff,
		stableAfter: m.stableAfter,
		runtimeDir:  m.runtimeDir,
		onStatus:    onStatus,
		secrets:     m.secretStoreLocked(hostID),
		prompter:    m.prompter,
		global:      m.globalOptions,
	})
	sess := &managed{watchdog: w, cancel: cancel, done: make(chan struct{})}
	m.sessions[hostID] = sess
	m.mu.Unlock()
	go func() {
		defer close(sess.done)
		w.loop(ctx)
	}()
	return nil
}

// Disconnect stops ssh for hostID and prevents reconnect.
func (m *Manager) Disconnect(hostID string) error {
	m.mu.Lock()
	sess, ok := m.sessions[hostID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("host %q is not connected", hostID)
	}
	delete(m.sessions, hostID)
	m.mu.Unlock()

	sess.watchdog.stop()
	sess.cancel()
	<-sess.done
	return nil
}

// Status is the current connection state for hostID.
func (m *Manager) Status(hostID string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[hostID]; ok {
		return sess.watchdog.snapshot()
	}
	// Unregistered hosts are simply disconnected — ListTunnels reads status
	// before Connect/syncHost, so "not found" must not surface as an Err.
	return Snapshot{Status: StatusDisconnected}
}

// Close disconnects every session.
func (m *Manager) Close() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Disconnect(id)
	}
}

// RecoverStaleSessions probes live sessions and bounces ssh children that look
// half-dead (typical after macOS sleep/wake). The watchdog reconnects without
// consuming retry budget. Returns how many sessions were bounced.
func (m *Manager) RecoverStaleSessions() int {
	m.mu.Lock()
	var stale []*watchdog
	for id, sess := range m.sessions {
		snap := sess.watchdog.snapshot()
		if snap.Status != StatusConnected {
			continue
		}
		if !sessionLooksStale(snap.PID, m.tunnels[id]) {
			continue
		}
		stale = append(stale, sess.watchdog)
	}
	m.mu.Unlock()

	for _, w := range stale {
		w.requestBounce()
	}
	return len(stale)
}
