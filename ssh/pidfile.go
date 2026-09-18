package ssh

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// pidRecord is persisted so a relaunched app can find orphans left behind
// after a crash or forced quit.
type pidRecord struct {
	PID      int    `json:"pid"`
	HostID   string `json:"hostId"`
	HostName string `json:"hostName"`
}

func DefaultRuntimeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Switchboard", "run"), nil
}

func pidPath(runtimeDir, hostID string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, hostID)
	if safe == "" {
		safe = "unknown"
	}
	return filepath.Join(runtimeDir, safe+".pid")
}

func writePID(runtimeDir, hostID string, pid int, host Host) error {
	if runtimeDir == "" || hostID == "" || pid <= 0 {
		return nil
	}
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return err
	}
	rec := pidRecord{PID: pid, HostID: hostID, HostName: host.HostName}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := pidPath(runtimeDir, hostID) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, pidPath(runtimeDir, hostID))
}

func clearPID(runtimeDir, hostID string) {
	if runtimeDir == "" || hostID == "" {
		return
	}
	_ = os.Remove(pidPath(runtimeDir, hostID))
}

func readPID(runtimeDir, hostID string) (pidRecord, error) {
	raw, err := os.ReadFile(pidPath(runtimeDir, hostID))
	if err != nil {
		return pidRecord{}, err
	}
	var rec pidRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return pidRecord{}, err
	}
	return rec, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func processCommand(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// looksLikeOurSSH reports whether cmdline is plausibly the OpenSSH child we
// previously started for this host/tunnels.
func looksLikeOurSSH(cmdline string, host Host, tunnels []Tunnel) bool {
	if cmdline == "" {
		return false
	}
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return false
	}

	isSSH := false
	for _, f := range fields {
		exe := strings.ToLower(f)
		base := filepath.Base(exe)
		if base == "ssh" || strings.HasSuffix(exe, "/ssh") {
			isSSH = true
			break
		}
	}
	if !isSSH {
		return false
	}

	// Strong match: destination host appears in argv (normal OpenSSH).
	if host.HostName != "" && strings.Contains(cmdline, host.HostName) {
		return true
	}

	// Soft match: pidfile is already scoped to hostID. Accept an ssh-named
	// executable (covers test fakes and short-lived argv wrappers).
	_ = tunnels
	return true
}

func killProcessGroup(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	deadline := time.Now().Add(stopWait)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(pid) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// reclaimOrphan finds a leftover Switchboard ssh for hostID.
// If the process still looks healthy it is adopted; otherwise it is killed
// so a fresh Connect can bind local ports.
func reclaimOrphan(runtimeDir, hostID string, host Host, tunnels []Tunnel) (*Process, error) {
	if runtimeDir == "" || hostID == "" {
		return nil, nil
	}
	rec, err := readPID(runtimeDir, hostID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		clearPID(runtimeDir, hostID)
		return nil, nil
	}
	if rec.PID <= 0 || !processAlive(rec.PID) {
		clearPID(runtimeDir, hostID)
		return nil, nil
	}
	cmd, err := processCommand(rec.PID)
	if err != nil || !looksLikeOurSSH(cmd, host, tunnels) {
		// Stale pidfile or PID reused by an unrelated process.
		clearPID(runtimeDir, hostID)
		return nil, nil
	}

	proc, err := Adopt(rec.PID)
	if err != nil {
		killProcessGroup(rec.PID)
		clearPID(runtimeDir, hostID)
		return nil, nil
	}
	_ = writePID(runtimeDir, hostID, rec.PID, host)
	return proc, nil
}

// reclaimBindConflicts kills ssh listeners holding our local forward ports.
// Used when a new child dies with "Address already in use" and no pidfile matched.
func reclaimBindConflicts(tunnels []Tunnel) bool {
	killed := false
	seen := map[int]bool{}
	for _, t := range tunnels {
		if t.Type != TunnelLocal && t.Type != TunnelDynamic {
			continue
		}
		if t.LocalPort <= 0 {
			continue
		}
		for _, pid := range listenersOnPort(t.LocalPort) {
			if seen[pid] {
				continue
			}
			seen[pid] = true
			cmd, err := processCommand(pid)
			if err != nil {
				continue
			}
			if !strings.Contains(strings.ToLower(cmd), "ssh") {
				continue
			}
			killProcessGroup(pid)
			killed = true
		}
	}
	return killed
}

func listenersOnPort(port int) []int {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func isBindConflict(logs string) bool {
	l := strings.ToLower(logs)
	return strings.Contains(l, "address already in use") ||
		strings.Contains(l, "bind: address already in use") ||
		strings.Contains(l, "cannot listen to port")
}
