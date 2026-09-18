package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const stopWait = 3 * time.Second

// Process is a running /usr/bin/ssh child. Stop sends SIGTERM to the
// process group; Status is derived from whether the process is still alive.
type Process struct {
	cmd  *exec.Cmd
	logs *syncBuffer

	done    chan struct{}
	waitErr error

	mu       sync.Mutex
	status   Status
	adopted  bool
	adoptPID int
}

// Start runs DefaultSSHBinary with SSHArgs(h, tunnels).
func Start(h Host, tunnels []Tunnel) (*Process, error) {
	return start(DefaultSSHBinary, h, tunnels, nil)
}

func start(sshBinary string, h Host, tunnels []Tunnel, extraEnv []string) (*Process, error) {
	if err := Validate(h, tunnels); err != nil {
		return nil, err
	}
	if sshBinary == "" {
		sshBinary = DefaultSSHBinary
	}

	args := withVerbose(SSHArgs(h, tunnels))
	configFile, env := splitConfigFromEnv(extraEnv)
	if configFile != "" {
		args = append([]string{"-F", configFile}, args...)
	}
	cmd := exec.Command(sshBinary, args...)
	logs := newSyncBuffer()
	cmd.Stdout = logs
	cmd.Stderr = logs
	if len(env) > 0 {
		cmd.Env = mergeEnv(env)
	}
	// New session so ssh has no controlling TTY and uses SSH_ASKPASS.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	null, err := os.Open(os.DevNull)
	if err == nil {
		cmd.Stdin = null
	}

	if err := cmd.Start(); err != nil {
		if null != nil {
			_ = null.Close()
		}
		return nil, fmt.Errorf("start ssh: %w", err)
	}
	if null != nil {
		_ = null.Close()
	}

	p := &Process{
		cmd:    cmd,
		logs:   logs,
		done:   make(chan struct{}),
		status: StatusConnecting,
	}
	go func() {
		p.waitErr = cmd.Wait()
		p.mu.Lock()
		if p.status != StatusDisconnected {
			if p.waitErr != nil {
				p.status = StatusError
			} else {
				p.status = StatusDisconnected
			}
		}
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

const sshConfigEnvKey = "SWITCHBOARD_SSH_CONFIG="

func splitConfigFromEnv(extraEnv []string) (configFile string, env []string) {
	env = make([]string, 0, len(extraEnv))
	for _, e := range extraEnv {
		if strings.HasPrefix(e, sshConfigEnvKey) {
			configFile = strings.TrimPrefix(e, sshConfigEnvKey)
			continue
		}
		env = append(env, e)
	}
	return configFile, env
}

// Adopt wraps an already-running ssh PID (typically left behind after an
// unexpected app quit) so the Manager can monitor and stop it.
func Adopt(pid int) (*Process, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid")
	}
	if !processAlive(pid) {
		return nil, fmt.Errorf("process %d is not running", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	p := &Process{
		cmd:      &exec.Cmd{Process: proc},
		logs:     newSyncBuffer(),
		done:     make(chan struct{}),
		status:   StatusConnected,
		adopted:  true,
		adoptPID: pid,
	}
	go func() {
		for processAlive(pid) {
			time.Sleep(400 * time.Millisecond)
		}
		p.mu.Lock()
		if p.status != StatusDisconnected {
			p.status = StatusDisconnected
		}
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

// Stop terminates the ssh process group and waits for it to exit.
func (p *Process) Stop() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	p.status = StatusDisconnected
	adopted := p.adopted
	adoptPID := p.adoptPID
	p.mu.Unlock()

	if adopted {
		killProcessGroup(adoptPID)
		// Reap if we are still the parent (test helpers / same-process spawn).
		// True crash orphans are reparented to launchd; Wait then returns ECHILD.
		if p.cmd != nil && p.cmd.Process != nil {
			_, _ = p.cmd.Process.Wait()
		}
		select {
		case <-p.done:
		case <-time.After(stopWait + time.Second):
		}
		return nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	pid := p.cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case <-p.done:
		return nil
	case <-time.After(stopWait):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-p.done
		return nil
	}
}

// Wait blocks until ssh exits.
func (p *Process) Wait() error {
	if p == nil {
		return nil
	}
	<-p.done
	return p.waitErr
}

// Status reports whether the child is still running.
func (p *Process) Status() Status {
	if p == nil {
		return StatusDisconnected
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.adopted {
		return p.status
	}
	if p.status == StatusConnecting {
		select {
		case <-p.done:
			if p.waitErr != nil {
				return StatusError
			}
			return StatusDisconnected
		default:
			return StatusConnecting
		}
	}
	return p.status
}

// PID is the ssh child pid, or 0.
func (p *Process) PID() int {
	if p == nil {
		return 0
	}
	if p.adopted {
		return p.adoptPID
	}
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Logs returns captured stdout and stderr.
func (p *Process) Logs() string {
	if p == nil || p.logs == nil {
		return ""
	}
	return p.logs.String()
}

func (p *Process) markConnected() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		return
	default:
		p.status = StatusConnected
	}
}

func mergeEnv(extra []string) []string {
	override := make(map[string]string, len(extra))
	order := make([]string, 0, len(extra))
	for _, e := range extra {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if _, exists := override[k]; !exists {
			order = append(order, k)
		}
		override[k] = v
	}
	base := os.Environ()
	out := make([]string, 0, len(base)+len(extra))
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if v, ok := override[k]; ok {
			out = append(out, k+"="+v)
			delete(override, k)
			continue
		}
		out = append(out, e)
	}
	for _, k := range order {
		if v, ok := override[k]; ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func withVerbose(args []string) []string {
	if len(args) > 0 && args[0] == "-N" {
		out := make([]string, 0, len(args)+1)
		out = append(out, "-N", "-v")
		return append(out, args[1:]...)
	}
	return append([]string{"-v"}, args...)
}

func authSucceeded(logs string) bool {
	return strings.Contains(logs, "Authenticated to") ||
		strings.Contains(logs, "Authentication succeeded")
}

func isAuthFailure(logs string) bool {
	l := strings.ToLower(logs)
	return strings.Contains(l, "permission denied") ||
		strings.Contains(l, "too many authentication failures") ||
		strings.Contains(l, "authentication failed")
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newSyncBuffer() *syncBuffer {
	return &syncBuffer{}
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() > 64*1024 {
		b.buf.Reset()
	}
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
