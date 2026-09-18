package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stenstromen/switchboard/config"
	"github.com/stenstromen/switchboard/keychain"
	"github.com/stenstromen/switchboard/ssh"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// PasswordPrompt is emitted when OpenSSH needs a secret.
type PasswordPrompt struct {
	RequestID string `json:"requestId"`
	HostID    string `json:"hostId"`
	Prompt    string `json:"prompt"`
	Kind      string `json:"kind"`
}

// StatusEvent is emitted whenever a tunnel's connection state changes.
type StatusEvent struct {
	HostID string       `json:"hostId"`
	Snap   ssh.Snapshot `json:"snap"`
}

// ConfigChangedEvent is emitted after import/export mutates saved tunnels or preferences.
type ConfigChangedEvent struct {
	Reason string `json:"reason"`
}

// TunnelView is a profile plus live status for the UI.
type TunnelView struct {
	Profile config.Profile `json:"profile"`
	Status  ssh.Snapshot   `json:"status"`
}

type notifyState struct {
	lastStatus     ssh.Status
	hadConnected   bool
	userDisconnect bool
}

// uiHooks are optional window/tray callbacks wired from AppUI.
type uiHooks interface {
	RefreshTray()
	RefreshMenuState()
	OpenSettings()
	OpenMain()
	ApplyShowAs(showAs config.ShowAs)
	SetSelection(id string)
}

// AppService is the Wails-facing API for tunnel configuration and control.
type AppService struct {
	store    *config.Store
	mgr      *ssh.Manager
	notifier *notifications.NotificationService
	ui       uiHooks

	mu      sync.Mutex
	pending map[string]*pendingPrompt
	notify  map[string]*notifyState
}

type pendingPrompt struct {
	ch     chan promptResult
	hostID string
	kind   ssh.SecretKind
}

type promptResult struct {
	secret string
	save   bool
	err    error
}

func NewAppService(store *config.Store, notifier *notifications.NotificationService) *AppService {
	s := &AppService{
		store:    store,
		notifier: notifier,
		pending:  make(map[string]*pendingPrompt),
		notify:   make(map[string]*notifyState),
	}
	s.mgr = ssh.NewManager(
		ssh.WithOnStatus(s.handleStatus),
		ssh.WithPrompter(s.prompt),
		ssh.WithBackoff(ssh.Backoff{Initial: time.Second, Max: time.Minute}),
	)
	s.syncSSHRuntimeFromStore()
	return s
}

// setUI attaches window/tray hooks after the UI is constructed.
func (s *AppService) setUI(ui uiHooks) {
	s.ui = ui
}

// ServiceStartup runs after the application is ready.
func (s *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.syncAllHosts()
	// Do not auto-prompt for notification permission — that is done from Options.
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(750 * time.Millisecond):
		}
		s.connectAutoConnect()
	}()
	return nil
}

func (s *AppService) syncAllHosts() {
	doc, err := s.store.Load()
	if err != nil {
		return
	}
	for _, p := range doc.Profiles {
		if p.HostName == "" {
			continue
		}
		_ = s.syncHost(p)
	}
}

func (s *AppService) connectAutoConnect() {
	doc, err := s.store.Load()
	if err != nil {
		return
	}
	for _, p := range doc.Profiles {
		if !p.AutoConnect {
			continue
		}
		if p.HostName == "" {
			continue
		}
		_ = s.Connect(p.ID)
	}
}

func (s *AppService) handleStatus(hostID string, snap ssh.Snapshot) {
	if app := application.Get(); app != nil {
		app.Event.Emit("tunnel:status", StatusEvent{HostID: hostID, Snap: snap})
	}
	s.emitLifecycleNotification(hostID, snap)
	if s.ui != nil {
		s.ui.RefreshTray()
		s.ui.RefreshMenuState()
	}
}

func (s *AppService) emitLifecycleNotification(hostID string, snap ssh.Snapshot) {
	p, ok, err := s.store.Get(hostID)
	if err != nil || !ok {
		s.trackStatus(hostID, snap.Status, false)
		return
	}

	s.mu.Lock()
	st := s.notifyStateLocked(hostID)
	prev := st.lastStatus
	hadConnected := st.hadConnected
	userDisconnect := st.userDisconnect
	if snap.Status == ssh.StatusConnected {
		st.hadConnected = true
		st.userDisconnect = false
	}
	st.lastStatus = snap.Status
	s.mu.Unlock()

	if !p.NotifyOnStatusChange {
		return
	}

	name := p.Name
	if name == "" {
		name = p.HostName
	}
	if name == "" {
		name = "Tunnel"
	}

	id := lifecycleNotifyID(hostID)

	switch snap.Status {
	case ssh.StatusConnecting:
		if hadConnected && !userDisconnect {
			// Already announced on the disconnect transition; only fill in if
			// we skipped straight to Connecting.
			if prev != ssh.StatusDisconnected && prev != ssh.StatusError {
				s.upsertNotification(id, name, "Connection lost, reconnecting…")
			}
		} else if prev != ssh.StatusConnecting {
			s.upsertNotification(id, name, "Connecting…")
		}
	case ssh.StatusConnected:
		// Same identifier replaces the Connecting / Reconnecting banner.
		s.upsertNotification(id, name, "Connected")
	case ssh.StatusError, ssh.StatusDisconnected:
		if userDisconnect {
			s.clearNotification(id)
			return
		}
		if hadConnected && prev == ssh.StatusConnected {
			s.upsertNotification(id, name, "Connection lost, reconnecting…")
		}
	}
}

func lifecycleNotifyID(hostID string) string {
	return "switchboard-tunnel-" + hostID
}

func (s *AppService) trackStatus(hostID string, status ssh.Status, userDisconnect bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.notifyStateLocked(hostID)
	if userDisconnect {
		st.userDisconnect = true
	}
	if status == ssh.StatusConnected {
		st.hadConnected = true
		st.userDisconnect = false
	}
	st.lastStatus = status
}

func (s *AppService) notifyStateLocked(hostID string) *notifyState {
	if s.notify[hostID] == nil {
		s.notify[hostID] = &notifyState{}
	}
	return s.notify[hostID]
}

// upsertNotification posts or replaces a delivered notification with the same ID.
// On macOS, UNUserNotificationCenter replaces by identifier — the recommended
// pattern for transient status updates (Connecting → Connected).
func (s *AppService) upsertNotification(id, title, body string) {
	if s.notifier != nil {
		opts := notifications.NotificationOptions{
			ID:    id,
			Title: title,
			Body:  body,
		}
		if err := s.notifier.UpdateNotification(opts); err == nil {
			return
		}
		if err := s.notifier.SendNotification(opts); err == nil {
			return
		}
	}
	// Fallback for unsigned / `go run` builds — cannot clear these later.
	script := fmt.Sprintf(
		`display notification %s with title %s`,
		appleScriptString(body),
		appleScriptString(title),
	)
	_ = exec.Command("osascript", "-e", script).Start()
}

func (s *AppService) clearNotification(id string) {
	if id == "" || s.notifier == nil {
		return
	}
	_ = s.notifier.RemoveDeliveredNotification(id)
	_ = s.notifier.RemovePendingNotification(id)
}

func appleScriptString(s string) string {
	out := make([]rune, 0, len(s)+2)
	out = append(out, '"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			out = append(out, '\\', r)
		default:
			out = append(out, r)
		}
	}
	out = append(out, '"')
	return string(out)
}

func (s *AppService) prompt(ctx context.Context, req ssh.PromptRequest) (string, error) {
	// If we're prompting, any Keychain value for this kind was already tried
	// (via hydrateSecrets) and rejected — drop the stale item.
	_ = keychain.Delete(req.HostID, keychain.Kind(req.Kind))

	id := uuid.NewString()
	ch := make(chan promptResult, 1)
	s.mu.Lock()
	s.pending[id] = &pendingPrompt{ch: ch, hostID: req.HostID, kind: req.Kind}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	application.Get().Event.Emit("tunnel:password", PasswordPrompt{
		RequestID: id,
		HostID:    req.HostID,
		Prompt:    req.Prompt,
		Kind:      string(req.Kind),
	})
	if s.ui != nil {
		s.ui.OpenMain()
	}

	select {
	case <-ctx.Done():
		return "", ssh.ErrPromptCancelled
	case res := <-ch:
		if res.err != nil {
			return "", res.err
		}
		if res.save && res.secret != "" {
			_ = keychain.Set(req.HostID, keychain.Kind(req.Kind), res.secret)
		}
		return res.secret, nil
	}
}

// SubmitPassword answers a PasswordPrompt. Empty secret cancels.
// When save is true, the secret is written to the macOS Keychain for this tunnel.
func (s *AppService) SubmitPassword(requestID, secret string, save bool) {
	s.mu.Lock()
	p := s.pending[requestID]
	s.mu.Unlock()
	if p == nil {
		return
	}
	if secret == "" {
		p.ch <- promptResult{err: ssh.ErrPromptCancelled}
		return
	}
	p.ch <- promptResult{secret: secret, save: save}
}

// ListTunnels returns every saved profile with live status.
func (s *AppService) ListTunnels() ([]TunnelView, error) {
	doc, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	out := make([]TunnelView, 0, len(doc.Profiles))
	for _, p := range doc.Profiles {
		if p.HostName != "" {
			_ = s.syncHost(p)
		}
		out = append(out, TunnelView{Profile: p, Status: s.mgr.Status(p.ID)})
	}
	return out, nil
}

// GetTunnel returns one profile + status.
func (s *AppService) GetTunnel(id string) (TunnelView, error) {
	p, ok, err := s.store.Get(id)
	if err != nil {
		return TunnelView{}, err
	}
	if !ok {
		return TunnelView{}, fmt.Errorf("tunnel %q not found", id)
	}
	return TunnelView{Profile: p, Status: s.mgr.Status(id)}, nil
}

// NewTunnelDraft returns a blank profile template (not yet saved).
func (s *AppService) NewTunnelDraft() config.Profile {
	p := config.NewProfile()
	p.ID = uuid.NewString()
	return p
}

// SaveTunnel creates or updates a profile and syncs it into the SSH manager.
// If the tunnel is live and connection settings changed (forwards, host, etc.),
// the SSH session is restarted so the new args take effect.
func (s *AppService) SaveTunnel(p config.Profile) (TunnelView, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.Name == "" {
		p.Name = "Unnamed"
	}
	if p.Port == 0 {
		p.Port = 22
	}

	old, hadOld, _ := s.store.Get(p.ID)
	prev := s.mgr.Status(p.ID)
	wasLive := prev.Status == ssh.StatusConnected || prev.Status == ssh.StatusConnecting
	needsRestart := wasLive && (!hadOld || connectionConfigChanged(old, p))

	if _, err := s.store.Upsert(p); err != nil {
		return TunnelView{}, err
	}

	if needsRestart {
		// Intentional restart — don't treat as an unexpected drop.
		s.trackStatus(p.ID, ssh.StatusDisconnected, true)
		_ = s.mgr.Disconnect(p.ID)
	}

	if err := s.syncHost(p); err != nil {
		return TunnelView{}, err
	}

	if needsRestart {
		if err := s.Connect(p.ID); err != nil {
			if s.ui != nil {
				s.ui.RefreshTray()
			}
			return TunnelView{Profile: p, Status: s.mgr.Status(p.ID)}, err
		}
	}

	if s.ui != nil {
		s.ui.RefreshTray()
	}
	return TunnelView{Profile: p, Status: s.mgr.Status(p.ID)}, nil
}

func connectionConfigChanged(old, next config.Profile) bool {
	if old.RetryAttempts != next.RetryAttempts {
		return true
	}
	a := ssh.SSHArgs(old.ToHost(), old.ToTunnels())
	b := ssh.SSHArgs(next.ToHost(), next.ToTunnels())
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

// DeleteTunnel removes a profile and disconnects it.
func (s *AppService) DeleteTunnel(id string) error {
	s.trackStatus(id, ssh.StatusDisconnected, true)
	_ = s.mgr.Disconnect(id)
	s.mgr.ClearSecrets(id)
	_ = keychain.DeleteAll(id)
	_, err := s.store.Delete(id)
	if s.ui != nil {
		s.ui.RefreshTray()
	}
	return err
}

// Connect starts the SSH session for a saved tunnel.
func (s *AppService) Connect(id string) error {
	p, ok, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tunnel %q not found", id)
	}
	s.mu.Lock()
	st := s.notifyStateLocked(id)
	st.userDisconnect = false
	s.mu.Unlock()
	if err := s.syncHost(p); err != nil {
		return err
	}
	s.hydrateSecrets(id)
	return s.mgr.Connect(id)
}

// Disconnect stops the SSH session.
func (s *AppService) Disconnect(id string) error {
	s.trackStatus(id, ssh.StatusDisconnected, true)
	s.clearNotification(lifecycleNotifyID(id))
	err := s.mgr.Disconnect(id)
	if s.ui != nil {
		s.ui.RefreshTray()
	}
	return err
}

// ConnectAll starts every saved tunnel.
func (s *AppService) ConnectAll() error {
	doc, err := s.store.Load()
	if err != nil {
		return err
	}
	var first error
	for _, p := range doc.Profiles {
		if err := s.Connect(p.ID); err != nil && first == nil {
			first = err
		}
	}
	if s.ui != nil {
		s.ui.RefreshTray()
	}
	return first
}

// DisconnectAll stops every tunnel session.
func (s *AppService) DisconnectAll() error {
	doc, err := s.store.Load()
	if err != nil {
		return err
	}
	var first error
	for _, p := range doc.Profiles {
		if err := s.Disconnect(p.ID); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// OpenSettings shows the Settings window.
func (s *AppService) OpenSettings() {
	if s.ui != nil {
		s.ui.OpenSettings()
	}
}

// OpenMainWindow shows the main Switchboard window.
func (s *AppService) OpenMainWindow() {
	if s.ui != nil {
		s.ui.OpenMain()
	}
}

// SetSelection tells the native menu which tunnel is currently selected.
func (s *AppService) SetSelection(id string) {
	if s.ui != nil {
		s.ui.SetSelection(id)
	}
}

// GetPreferences returns app-wide settings. OpenAtLogin reflects the live
// autostart registration when available.
func (s *AppService) GetPreferences() config.Preferences {
	prefs, err := s.store.Preferences()
	if err != nil {
		prefs = config.DefaultPreferences()
	}
	if app := application.Get(); app != nil && app.Autostart != nil {
		if enabled, err := app.Autostart.IsEnabled(); err == nil {
			prefs.OpenAtLogin = enabled
		}
	}
	return prefs
}

// SetPreferences persists settings and applies Show As / Open at Login / SSH runtime.
func (s *AppService) SetPreferences(prefs config.Preferences) (config.Preferences, error) {
	prefs = prefs.Normalize()
	old, _ := s.store.Preferences()
	if err := s.store.SetPreferences(prefs); err != nil {
		return prefs, err
	}
	runtimeChanged := config.SSHRuntimeChanged(old, prefs)
	s.syncSSHRuntime(prefs)

	if err := s.applyOpenAtLogin(prefs.OpenAtLogin); err != nil {
		if s.ui != nil {
			s.ui.ApplyShowAs(prefs.ShowAs)
		}
		return s.GetPreferences(), err
	}
	if s.ui != nil {
		s.ui.ApplyShowAs(prefs.ShowAs)
	}
	if runtimeChanged {
		s.restartLiveTunnels()
	}
	return s.GetPreferences(), nil
}

// ConfigTransferResult summarizes an export or import.
type ConfigTransferResult struct {
	Path         string `json:"path"`
	ProfileCount int    `json:"profileCount"`
	Added        int    `json:"added,omitempty"`
	Updated      int    `json:"updated,omitempty"`
	Removed      int    `json:"removed,omitempty"`
	Message      string `json:"message"`
}

// ExportConfiguration writes tunnels + preferences to path (JSON).
// Keychain passwords/passphrases are never included.
func (s *AppService) ExportConfiguration(path string) (ConfigTransferResult, error) {
	if path == "" {
		return ConfigTransferResult{}, fmt.Errorf("no path selected")
	}
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		path += ".json"
	}
	doc, err := s.store.Load()
	if err != nil {
		return ConfigTransferResult{}, err
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return ConfigTransferResult{}, err
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return ConfigTransferResult{}, err
	}
	n := len(doc.Profiles)
	return ConfigTransferResult{
		Path:         path,
		ProfileCount: n,
		Message:      fmt.Sprintf("Exported %d tunnel%s (passwords stay in Keychain).", n, pluralS(n)),
	}, nil
}

// ImportConfiguration loads tunnels + preferences from path.
// replace=false merges by profile id; replace=true swaps the full tunnel list.
// Keychain secrets are never imported. On replace, Keychain items for removed tunnels are deleted.
func (s *AppService) ImportConfiguration(path string, replace bool) (ConfigTransferResult, error) {
	if path == "" {
		return ConfigTransferResult{}, fmt.Errorf("no path selected")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConfigTransferResult{}, err
	}
	var incoming config.Document
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return ConfigTransferResult{}, fmt.Errorf("invalid configuration file: %w", err)
	}
	if incoming.Profiles == nil {
		incoming.Profiles = []config.Profile{}
	}
	for i := range incoming.Profiles {
		if incoming.Profiles[i].Port == 0 {
			incoming.Profiles[i].Port = 22
		}
		if incoming.Profiles[i].Forwards == nil {
			incoming.Profiles[i].Forwards = []config.Forward{}
		}
		if incoming.Profiles[i].ID == "" {
			incoming.Profiles[i].ID = uuid.NewString()
		}
	}

	current, err := s.store.Load()
	if err != nil {
		return ConfigTransferResult{}, err
	}

	var added, updated, removed int
	var nextProfiles []config.Profile

	if replace {
		oldIDs := map[string]struct{}{}
		for _, p := range current.Profiles {
			oldIDs[p.ID] = struct{}{}
		}
		nextProfiles = append([]config.Profile(nil), incoming.Profiles...)
		newIDs := map[string]struct{}{}
		for _, p := range nextProfiles {
			newIDs[p.ID] = struct{}{}
		}
		for id := range oldIDs {
			if _, keep := newIDs[id]; !keep {
				_ = s.mgr.Disconnect(id)
				s.mgr.ClearSecrets(id)
				_ = keychain.DeleteAll(id)
				removed++
			}
		}
		added = len(nextProfiles)
	} else {
		byID := map[string]int{}
		nextProfiles = append([]config.Profile(nil), current.Profiles...)
		for i, p := range nextProfiles {
			byID[p.ID] = i
		}
		for _, p := range incoming.Profiles {
			if i, ok := byID[p.ID]; ok {
				nextProfiles[i] = p
				updated++
			} else {
				nextProfiles = append(nextProfiles, p)
				byID[p.ID] = len(nextProfiles) - 1
				added++
			}
		}
	}

	current.Profiles = nextProfiles
	if incoming.Preferences != nil {
		prefs := incoming.Preferences.Normalize()
		current.Preferences = &prefs
	}
	if err := s.store.Save(current); err != nil {
		return ConfigTransferResult{}, err
	}

	if current.Preferences != nil {
		s.syncSSHRuntime(*current.Preferences)
		if s.ui != nil {
			s.ui.ApplyShowAs(current.Preferences.ShowAs)
		}
		_ = s.applyOpenAtLogin(current.Preferences.OpenAtLogin)
	}
	s.syncAllHosts()
	if s.ui != nil {
		s.ui.RefreshTray()
		s.ui.RefreshMenuState()
	}
	s.emitConfigChanged("import")

	n := len(nextProfiles)
	msg := fmt.Sprintf("Imported %d tunnel%s.", n, pluralS(n))
	if !replace {
		msg = fmt.Sprintf("Merged configuration: %d added, %d updated.", added, updated)
	} else if removed > 0 {
		msg = fmt.Sprintf("Replaced configuration with %d tunnel%s (%d removed).", n, pluralS(n), removed)
	}

	return ConfigTransferResult{
		Path:         path,
		ProfileCount: n,
		Added:        added,
		Updated:      updated,
		Removed:      removed,
		Message:      msg,
	}, nil
}

func (s *AppService) emitConfigChanged(reason string) {
	if app := application.Get(); app != nil {
		app.Event.Emit("config:changed", ConfigChangedEvent{Reason: reason})
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (s *AppService) syncSSHRuntimeFromStore() {
	prefs, err := s.store.Preferences()
	if err != nil {
		prefs = config.DefaultPreferences()
	}
	s.syncSSHRuntime(prefs)
}

func (s *AppService) syncSSHRuntime(prefs config.Preferences) {
	s.mgr.SetGlobalOptions(ssh.GlobalOptions{
		AuthSock:   prefs.AuthAgent, // empty = inherit process default
		ConfigFile: prefs.ConfigFile, // empty = OpenSSH default ~/.ssh/config
		Env:        prefs.EnabledEnvPairs(),
	})
}

func (s *AppService) restartLiveTunnels() {
	doc, err := s.store.Load()
	if err != nil {
		return
	}
	for _, p := range doc.Profiles {
		st := s.mgr.Status(p.ID).Status
		if st != ssh.StatusConnected && st != ssh.StatusConnecting {
			continue
		}
		s.trackStatus(p.ID, ssh.StatusDisconnected, true)
		_ = s.mgr.Disconnect(p.ID)
		_ = s.Connect(p.ID)
	}
	if s.ui != nil {
		s.ui.RefreshTray()
	}
}

// SSHDefaults are resolved default paths shown when overrides are empty.
type SSHDefaults struct {
	AuthAgent  string `json:"authAgent"`
	ConfigFile string `json:"configFile"`
	KnownHosts string `json:"knownHosts"`
}

// GetSSHDefaults returns system defaults for SSH location fields.
func (s *AppService) GetSSHDefaults() SSHDefaults {
	return SSHDefaults{
		AuthAgent:  config.DefaultAuthAgent(),
		ConfigFile: config.DefaultConfigFile(),
		KnownHosts: config.KnownHostsPath(),
	}
}

// ListKnownHosts returns parsed entries from ~/.ssh/known_hosts.
func (s *AppService) ListKnownHosts() ([]config.KnownHostEntry, error) {
	return config.ListKnownHosts("")
}

// AddKnownHost appends a full known_hosts line.
func (s *AppService) AddKnownHost(line string) error {
	return config.AddKnownHostLine("", line)
}

// RemoveKnownHost deletes the entry at index from ListKnownHosts.
func (s *AppService) RemoveKnownHost(index int) error {
	return config.RemoveKnownHostAt("", index)
}

// RevealPathInFinder shows path in Finder (macOS).
func (s *AppService) RevealPathInFinder(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return exec.Command("open", "-R", path).Start()
}

// OpenPathInEditor opens path in the default text editor.
func (s *AppService) OpenPathInEditor(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return exec.Command("open", "-t", path).Start()
}

func (s *AppService) applyOpenAtLogin(enabled bool) error {
	app := application.Get()
	if app == nil || app.Autostart == nil {
		return nil
	}
	if enabled {
		return app.Autostart.Enable()
	}
	return app.Autostart.Disable()
}

// PreviewArgs returns the OpenSSH argument slice for a profile (debug/help).
func (s *AppService) PreviewArgs(p config.Profile) []string {
	return ssh.SSHArgs(p.ToHost(), p.ToTunnels())
}

// SetPassword caches a login password for reconnects and stores it in Keychain.
func (s *AppService) SetPassword(id, password string) error {
	p, ok, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tunnel %q not found", id)
	}
	if err := s.syncHost(p); err != nil {
		return err
	}
	if err := keychain.Set(id, keychain.KindPassword, password); err != nil {
		return err
	}
	return s.mgr.SetPassword(id, password)
}

// ClearSavedPassword removes the Keychain password for a tunnel.
func (s *AppService) ClearSavedPassword(id string) error {
	if err := keychain.Delete(id, keychain.KindPassword); err != nil {
		return err
	}
	_ = s.mgr.SetPassword(id, "")
	return nil
}

func (s *AppService) hydrateSecrets(id string) {
	if pw, err := keychain.Get(id, keychain.KindPassword); err == nil && pw != "" {
		_ = s.mgr.SetPassword(id, pw)
	}
	if pp, err := keychain.Get(id, keychain.KindPassphrase); err == nil && pp != "" {
		_ = s.mgr.SetPassphrase(id, pp)
	}
}

func (s *AppService) syncHost(p config.Profile) error {
	if err := s.mgr.AddHost(p.ToHost()); err != nil {
		return err
	}
	return s.mgr.SetTunnels(p.ID, p.ToTunnels())
}

// NotificationPermission describes macOS notification authorization state.
type NotificationPermission struct {
	Authorized bool   `json:"authorized"`
	Available  bool   `json:"available"`
	Message    string `json:"message"`
}

// GetNotificationPermission returns the current notification authorization state.
func (s *AppService) GetNotificationPermission() NotificationPermission {
	if s.notifier == nil {
		return NotificationPermission{
			Available: false,
			Message:   "Notification service is not available.",
		}
	}
	ok, err := s.notifier.CheckNotificationAuthorization()
	if err != nil {
		return NotificationPermission{
			Available: true,
			Message:   err.Error(),
		}
	}
	if ok {
		return NotificationPermission{
			Authorized: true,
			Available:  true,
			Message:    "Notifications are allowed.",
		}
	}
	return NotificationPermission{
		Authorized: false,
		Available:  true,
		Message:    "Notifications are not allowed yet. Click Enable to request permission.",
	}
}

// RequestNotificationPermission prompts macOS for notification access.
func (s *AppService) RequestNotificationPermission() (NotificationPermission, error) {
	if s.notifier == nil {
		return NotificationPermission{
			Available: false,
			Message:   "Notification service is not available.",
		}, fmt.Errorf("notification service unavailable")
	}
	ok, err := s.notifier.RequestNotificationAuthorization()
	if err != nil {
		return NotificationPermission{
			Available: true,
			Message:   err.Error(),
		}, err
	}
	if ok {
		return NotificationPermission{
			Authorized: true,
			Available:  true,
			Message:    "Notifications are allowed.",
		}, nil
	}
	return NotificationPermission{
		Authorized: false,
		Available:  true,
		Message:    "Permission was not granted. You can enable Switchboard in System Settings → Notifications.",
	}, nil
}

// SendTestNotification delivers a sample notification so permission can be verified.
func (s *AppService) SendTestNotification() error {
	perm := s.GetNotificationPermission()
	if !perm.Authorized {
		return fmt.Errorf("notifications are not authorized")
	}
	s.upsertNotification(
		fmt.Sprintf("switchboard-test-%d", time.Now().UnixNano()),
		"Switchboard",
		"Test notification — you’re all set.",
	)
	return nil
}

func (s *AppService) Close() {
	s.mgr.Close()
}
