package ssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Backoff controls how long the watchdog waits between reconnect attempts.
type Backoff struct {
	Initial time.Duration
	Max     time.Duration
}

func (b Backoff) withDefaults() Backoff {
	if b.Initial <= 0 {
		b.Initial = time.Second
	}
	if b.Max <= 0 {
		b.Max = time.Minute
	}
	if b.Max < b.Initial {
		b.Max = b.Initial
	}
	return b
}

func (b Backoff) next(current time.Duration) time.Duration {
	if current <= 0 {
		return b.Initial
	}
	next := current * 2
	if next > b.Max {
		return b.Max
	}
	return next
}

type watchdogConfig struct {
	host        Host
	hostID      string
	tunnels     []Tunnel
	sshBinary   string
	askpassBin  string
	backoff     Backoff
	stableAfter time.Duration
	runtimeDir  string
	onStatus    func(Snapshot)
	secrets     *secretStore
	prompter    Prompter
	global      func() GlobalOptions
}

// watchdog owns one Host's ssh child: detect exit, reconnect, backoff.
type watchdog struct {
	host        Host
	hostID      string
	tunnels     []Tunnel
	sshBinary   string
	askpassBin  string
	backoff     Backoff
	stableAfter time.Duration
	runtimeDir  string
	onStatus    func(Snapshot)
	secrets     *secretStore
	prompter    Prompter
	global      func() GlobalOptions
	maxRetries  int // from Host.MaxRetries; negative = unlimited

	mu      sync.Mutex
	proc    *Process
	status  Status
	lastErr string
	stopped bool
}

func newWatchdog(cfg watchdogConfig) *watchdog {
	if cfg.stableAfter <= 0 {
		cfg.stableAfter = 300 * time.Millisecond
	}
	if cfg.secrets == nil {
		cfg.secrets = newSecretStore()
	}
	if cfg.global == nil {
		cfg.global = func() GlobalOptions { return GlobalOptions{} }
	}
	hostID := cfg.hostID
	if hostID == "" {
		hostID = cfg.host.id()
	}
	return &watchdog{
		host:        cfg.host,
		hostID:      hostID,
		tunnels:     append([]Tunnel(nil), cfg.tunnels...),
		sshBinary:   cfg.sshBinary,
		askpassBin:  cfg.askpassBin,
		backoff:     cfg.backoff.withDefaults(),
		stableAfter: cfg.stableAfter,
		runtimeDir:  cfg.runtimeDir,
		onStatus:    cfg.onStatus,
		secrets:     cfg.secrets,
		prompter:    cfg.prompter,
		global:      cfg.global,
		maxRetries:  cfg.host.MaxRetries,
		status:      StatusConnecting,
	}
}

func (w *watchdog) loop(ctx context.Context) {
	askpassBin, err := resolveAskpass(w.askpassBin)
	if err != nil {
		w.set(StatusError, err.Error(), 0)
		return
	}
	srv, err := startAskpassServer(ctx, w.hostID, w.secrets, w.prompter)
	if err != nil {
		w.set(StatusError, err.Error(), 0)
		return
	}
	defer srv.close()

	delay := time.Duration(0)
	failures := 0
	for {
		if w.isStopped() {
			w.set(StatusDisconnected, "", 0)
			return
		}

		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				w.set(StatusDisconnected, "", 0)
				return
			case <-timer.C:
			}
		}

		if w.isStopped() {
			w.set(StatusDisconnected, "", 0)
			return
		}

		w.secrets.nextAttempt()
		w.set(StatusConnecting, "", 0)

		// Prefer adopting an orphan left behind after an unexpected quit so we
		// don't fight it for local listen ports (and avoid a reconnect storm).
		if adopted, _ := reclaimOrphan(w.runtimeDir, w.hostID, w.host, w.tunnels); adopted != nil {
			if !w.setProc(adopted) {
				_ = adopted.Stop()
				clearPID(w.runtimeDir, w.hostID)
				w.set(StatusDisconnected, "", 0)
				return
			}
			failures = 0
			w.set(StatusConnected, "", adopted.PID())
			select {
			case <-ctx.Done():
				_ = adopted.Stop()
				w.clearProc(adopted)
				clearPID(w.runtimeDir, w.hostID)
				w.set(StatusDisconnected, "", 0)
				return
			case <-adopted.done:
				if w.isStopped() {
					w.clearProc(adopted)
					clearPID(w.runtimeDir, w.hostID)
					w.set(StatusDisconnected, "", 0)
					return
				}
				w.clearProc(adopted)
				clearPID(w.runtimeDir, w.hostID)
				w.set(StatusError, "adopted ssh exited", 0)
				failures++
				if w.retriesExhausted(failures) {
					return
				}
				delay = w.backoff.next(delay)
				continue
			}
		}

		proc, err := start(w.sshBinary, w.host, w.tunnels, w.childEnv(srv, askpassBin))
		if err != nil {
			w.set(StatusError, err.Error(), 0)
			failures++
			if w.retriesExhausted(failures) {
				return
			}
			delay = w.backoff.next(delay)
			continue
		}
		if !w.setProc(proc) {
			_ = proc.Stop()
			w.set(StatusDisconnected, "", 0)
			return
		}
		_ = writePID(w.runtimeDir, w.hostID, proc.PID(), w.host)

		stable, stop := w.awaitStable(ctx, proc, srv)
		if stop {
			_ = proc.Stop()
			w.clearProc(proc)
			clearPID(w.runtimeDir, w.hostID)
			w.set(StatusDisconnected, "", 0)
			return
		}
		if !stable {
			logs := proc.Logs()
			if w.abortOnAuth(proc) {
				clearPID(w.runtimeDir, w.hostID)
				return
			}
			w.clearProc(proc)
			if isBindConflict(logs) && reclaimBindConflicts(w.tunnels) {
				clearPID(w.runtimeDir, w.hostID)
				delay = 0
				continue
			}
			clearPID(w.runtimeDir, w.hostID)
			failures++
			if w.retriesExhausted(failures) {
				return
			}
			delay = w.backoff.next(delay)
			continue
		}

		failures = 0
		delay = 0
		select {
		case <-ctx.Done():
			_ = proc.Stop()
			w.clearProc(proc)
			clearPID(w.runtimeDir, w.hostID)
			w.set(StatusDisconnected, "", 0)
			return
		case <-proc.done:
			if w.isStopped() {
				w.clearProc(proc)
				clearPID(w.runtimeDir, w.hostID)
				w.set(StatusDisconnected, "", 0)
				return
			}
			logs := proc.Logs()
			if w.abortOnAuth(proc) {
				clearPID(w.runtimeDir, w.hostID)
				return
			}
			msg := logs
			if proc.waitErr != nil && msg == "" {
				msg = proc.waitErr.Error()
			}
			w.clearProc(proc)
			clearPID(w.runtimeDir, w.hostID)
			w.set(StatusError, msg, 0)
			if isBindConflict(msg) && reclaimBindConflicts(w.tunnels) {
				delay = 0
				continue
			}
			failures++
			if w.retriesExhausted(failures) {
				return
			}
			delay = w.backoff.next(delay)
		}
	}
}

// retriesExhausted is true when consecutive failures exceeded MaxRetries.
// MaxRetries is retries after failure (0 = try once). Negative = unlimited.
func (w *watchdog) retriesExhausted(failures int) bool {
	if w.maxRetries < 0 {
		return false
	}
	if failures > w.maxRetries {
		prev := w.snapshot().Err
		msg := fmt.Sprintf("stopped after %d failed attempt(s)", failures)
		if prev != "" {
			msg = prev + "; " + msg
		}
		w.set(StatusError, msg, 0)
		return true
	}
	return false
}

func (w *watchdog) abortOnAuth(proc *Process) bool {
	if w.secrets.cancelled() {
		w.clearProc(proc)
		w.set(StatusError, ErrPromptCancelled.Error(), 0)
		return true
	}
	logs := ""
	if proc != nil {
		logs = proc.Logs()
	}
	if isAuthFailure(logs) {
		w.secrets.clear()
		w.clearProc(proc)
		msg := strings.TrimSpace(logs)
		if msg == "" {
			msg = "permission denied"
		}
		w.set(StatusError, msg, 0)
		return true
	}
	return false
}

func (w *watchdog) awaitStable(ctx context.Context, proc *Process, srv *askpassServer) (stable, stop bool) {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()

	var activity <-chan struct{}
	if srv != nil {
		activity = srv.activity
	}

	for {
		select {
		case <-ctx.Done():
			return false, true
		case <-proc.done:
			return w.exitDuringWait(proc)
		case <-activity:
			continue
		case <-tick.C:
			if srv != nil && srv.busy() {
				continue
			}
			if authSucceeded(proc.Logs()) {
				return w.holdStable(ctx, proc)
			}
		}
	}
}

func (w *watchdog) holdStable(ctx context.Context, proc *Process) (stable, stop bool) {
	timer := time.NewTimer(w.stableAfter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, true
	case <-proc.done:
		return w.exitDuringWait(proc)
	case <-timer.C:
		proc.markConnected()
		w.set(StatusConnected, "", proc.PID())
		return true, false
	}
}

func (w *watchdog) exitDuringWait(proc *Process) (stable, stop bool) {
	if w.isStopped() {
		return false, true
	}
	msg := proc.Logs()
	if proc.waitErr != nil && msg == "" {
		msg = proc.waitErr.Error()
	}
	w.set(StatusError, msg, proc.PID())
	return false, false
}

func (w *watchdog) childEnv(srv *askpassServer, askpassBin string) []string {
	env := srv.childEnv(askpassBin)
	g := w.global()
	if g.AuthSock != "" {
		env = append(env, "SSH_AUTH_SOCK="+g.AuthSock)
	}
	if g.ConfigFile != "" {
		// OpenSSH reads -F from argv; pass via SWITCHBOARD marker consumed in start.
		env = append(env, "SWITCHBOARD_SSH_CONFIG="+g.ConfigFile)
	}
	env = append(env, g.Env...)
	return env
}

func (w *watchdog) stop() {
	w.mu.Lock()
	w.stopped = true
	proc := w.proc
	w.mu.Unlock()
	_ = proc.Stop()
	clearPID(w.runtimeDir, w.hostID)
}

func (w *watchdog) isStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func (w *watchdog) setProc(proc *Process) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return false
	}
	w.proc = proc
	return true
}

func (w *watchdog) clearProc(proc *Process) {
	w.mu.Lock()
	if w.proc == proc {
		w.proc = nil
	}
	w.mu.Unlock()
}

func (w *watchdog) set(status Status, errMsg string, pid int) {
	w.mu.Lock()
	w.status = status
	w.lastErr = errMsg
	onStatus := w.onStatus
	logs := ""
	if w.proc != nil {
		logs = w.proc.Logs()
		if pid == 0 {
			pid = w.proc.PID()
		}
	}
	snap := Snapshot{Status: status, Err: errMsg, PID: pid, Logs: logs}
	w.mu.Unlock()
	if onStatus != nil {
		onStatus(snap)
	}
}

func (w *watchdog) snapshot() Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	snap := Snapshot{Status: w.status, Err: w.lastErr}
	if w.proc != nil {
		snap.PID = w.proc.PID()
		snap.Logs = w.proc.Logs()
	}
	return snap
}

func resolveAskpass(path string) (string, error) {
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("askpass executable: %w", err)
		}
		path = exe
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}
