package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/stenstromen/switchboard/ssh"
	"golang.org/x/term"
)

func main() {
	if ssh.IsAskpass() {
		os.Exit(ssh.RunAskpass())
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "args":
		os.Exit(runArgs(os.Args[2:]))
	case "connect":
		os.Exit(runConnect(os.Args[2:]))
	case "askpass":
		os.Exit(ssh.RunAskpass())
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `switchboard — process manager around /usr/bin/ssh

Usage:
  switchboard args [flags]
  switchboard connect [flags]

OpenSSH asks for passwords and key passphrases through SSH_ASKPASS.
connect prompts on this terminal when ssh needs a secret.

Flags:
  -name string
  -host string
  -user string
  -port int
  -jump string
  -identity string
  -strict-host-key string    OpenSSH StrictHostKeyChecking (e.g. accept-new)
  -hash-known-hosts
  -keepalive int             ServerAliveInterval seconds
  -L spec                    local forward  [bind:]port:host:hostport (repeatable)
  -R spec                    remote forward [bind:]port:host:hostport (repeatable)
  -D spec                    dynamic forward [bind:]port (repeatable)
`)
}

func runArgs(argv []string) int {
	h, tunnels, err := parseFlags("args", argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := ssh.Validate(h, tunnels); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	args := ssh.SSHArgs(h, tunnels)
	fmt.Println(ssh.DefaultSSHBinary)
	for _, a := range args {
		fmt.Println(a)
	}
	return 0
}

func runConnect(argv []string) int {
	h, tunnels, err := parseFlags("connect", argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	mgr := ssh.NewManager(
		ssh.WithOnStatus(func(hostID string, snap ssh.Snapshot) {
			if snap.Err != "" {
				fmt.Fprintf(os.Stderr, "%s: %s (%s)\n", hostID, snap.Status, strings.TrimSpace(snap.Err))
				return
			}
			fmt.Fprintf(os.Stderr, "%s: %s\n", hostID, snap.Status)
		}),
		ssh.WithPrompter(terminalPrompter),
	)
	defer mgr.Close()

	if err := mgr.AddHost(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	id := h.Name
	if id == "" {
		id = h.HostName
	}
	for _, tun := range tunnels {
		if err := mgr.AddTunnel(id, tun); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	fmt.Fprintf(os.Stderr, "exec: %s %s\n", ssh.DefaultSSHBinary, strings.Join(ssh.SSHArgs(h, tunnels), " "))
	if err := mgr.Connect(id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	return 0
}

func terminalPrompter(ctx context.Context, req ssh.PromptRequest) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("password prompt requires a terminal: %w", err)
	}
	defer tty.Close()

	prompt := req.Prompt
	if strings.TrimSpace(prompt) == "" {
		prompt = "Password: "
	}

	type outcome struct {
		secret string
		err    error
	}
	ch := make(chan outcome, 1)
	go func() {
		_, _ = fmt.Fprint(tty, prompt)
		pw, err := term.ReadPassword(int(tty.Fd()))
		_, _ = fmt.Fprint(tty, "\n")
		ch <- outcome{string(pw), err}
	}()
	select {
	case <-ctx.Done():
		return "", ssh.ErrPromptCancelled
	case out := <-ch:
		if out.err != nil {
			return "", out.err
		}
		return out.secret, nil
	}
}

type specList []string

func (s *specList) String() string { return strings.Join(*s, ",") }
func (s *specList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func parseFlags(name string, argv []string) (ssh.Host, []ssh.Tunnel, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		hostName   string
		nameFlag   string
		user       string
		port       int
		jump       string
		identity   string
		strict     string
		hashKnown  bool
		keepalive  int
		localFwd   specList
		remoteFwd  specList
		dynamicFwd specList
	)

	fs.StringVar(&nameFlag, "name", "", "local host id")
	fs.StringVar(&hostName, "host", "", "SSH HostName")
	fs.StringVar(&user, "user", "", "SSH user")
	fs.IntVar(&port, "port", 0, "SSH port")
	fs.StringVar(&jump, "jump", "", "ProxyJump")
	fs.StringVar(&identity, "identity", "", "IdentityFile")
	fs.StringVar(&strict, "strict-host-key", "", "StrictHostKeyChecking")
	fs.BoolVar(&hashKnown, "hash-known-hosts", false, "HashKnownHosts=yes")
	fs.IntVar(&keepalive, "keepalive", 0, "ServerAliveInterval")
	fs.Var(&localFwd, "L", "local forward")
	fs.Var(&remoteFwd, "R", "remote forward")
	fs.Var(&dynamicFwd, "D", "dynamic forward")

	if err := fs.Parse(argv); err != nil {
		return ssh.Host{}, nil, err
	}

	h := ssh.Host{
		Name:                  nameFlag,
		HostName:              hostName,
		User:                  user,
		Port:                  port,
		ProxyJump:             jump,
		IdentityFile:          identity,
		StrictHostKeyChecking: strict,
		HashKnownHosts:        hashKnown,
	}
	if keepalive > 0 {
		h.ServerAliveInterval = keepalive
		h.ServerAliveCountMax = 3
	}

	var tunnels []ssh.Tunnel
	for _, spec := range localFwd {
		t, err := parseForward(ssh.TunnelLocal, spec)
		if err != nil {
			return ssh.Host{}, nil, err
		}
		tunnels = append(tunnels, t)
	}
	for _, spec := range remoteFwd {
		t, err := parseForward(ssh.TunnelRemote, spec)
		if err != nil {
			return ssh.Host{}, nil, err
		}
		tunnels = append(tunnels, t)
	}
	for _, spec := range dynamicFwd {
		t, err := parseDynamic(spec)
		if err != nil {
			return ssh.Host{}, nil, err
		}
		tunnels = append(tunnels, t)
	}
	return h, tunnels, nil
}

func parseForward(kind ssh.TunnelType, spec string) (ssh.Tunnel, error) {
	parts := strings.Split(spec, ":")
	var localHost, remoteHost string
	var localPort, remotePort int
	var err error

	switch len(parts) {
	case 3:
		localPort, err = strconv.Atoi(parts[0])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -L/-R %q", spec)
		}
		remoteHost = parts[1]
		remotePort, err = strconv.Atoi(parts[2])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -L/-R %q", spec)
		}
	case 4:
		localHost = parts[0]
		localPort, err = strconv.Atoi(parts[1])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -L/-R %q", spec)
		}
		remoteHost = parts[2]
		remotePort, err = strconv.Atoi(parts[3])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -L/-R %q", spec)
		}
	default:
		return ssh.Tunnel{}, fmt.Errorf("invalid -L/-R %q", spec)
	}

	return ssh.Tunnel{
		Type:       kind,
		LocalHost:  localHost,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
	}, nil
}

func parseDynamic(spec string) (ssh.Tunnel, error) {
	parts := strings.Split(spec, ":")
	switch len(parts) {
	case 1:
		port, err := strconv.Atoi(parts[0])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -D %q", spec)
		}
		return ssh.Tunnel{Type: ssh.TunnelDynamic, LocalPort: port}, nil
	case 2:
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return ssh.Tunnel{}, fmt.Errorf("invalid -D %q", spec)
		}
		return ssh.Tunnel{Type: ssh.TunnelDynamic, LocalHost: parts[0], LocalPort: port}, nil
	default:
		return ssh.Tunnel{}, fmt.Errorf("invalid -D %q", spec)
	}
}
