package ssh

import (
	"slices"
	"testing"
)

func TestSSHArgsMilestoneExample(t *testing.T) {
	h := Host{
		HostName:              "server.example.com",
		User:                  "filip",
		Port:                  2222,
		ProxyJump:             "jump.example.com",
		StrictHostKeyChecking: "accept-new",
		HashKnownHosts:        true,
		NoSession:             true,
	}

	got := SSHArgs(h, nil)
	want := []string{
		"-N",
		"-p", "2222",
		"-o", "ProxyJump=jump.example.com",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "HashKnownHosts=yes",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		"filip@server.example.com",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SSHArgs() =\n%q\nwant\n%q", got, want)
	}
}

func TestSSHArgsOmitsZeroPortAndEmptyOptions(t *testing.T) {
	got := SSHArgs(Host{HostName: "example.com", NoSession: true}, nil)
	want := []string{
		"-N",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		"example.com",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SSHArgs() = %q, want %q", got, want)
	}
}

func TestSSHArgsTunnels(t *testing.T) {
	h := Host{HostName: "server.example.com", User: "filip", NoSession: true}
	tunnels := []Tunnel{
		{Type: TunnelLocal, LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 80},
		{Type: TunnelRemote, LocalHost: "127.0.0.1", LocalPort: 9090, RemoteHost: "localhost", RemotePort: 9090},
		{Type: TunnelDynamic, LocalPort: 1080},
	}

	got := SSHArgs(h, tunnels)
	want := []string{
		"-N",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "8080:127.0.0.1:80",
		"-R", "127.0.0.1:9090:localhost:9090",
		"-D", "1080",
		"filip@server.example.com",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SSHArgs() =\n%q\nwant\n%q", got, want)
	}
}

func TestSSHArgsKeepaliveAndIdentity(t *testing.T) {
	h := Host{
		HostName:            "example.com",
		IdentityFile:        "/tmp/id_ed25519",
		CertificateFile:     "/tmp/id_ed25519-cert.pub",
		ServerAliveInterval: 30,
		ServerAliveCountMax: 3,
		ForwardAgent:        true,
		NoSession:           true,
	}
	got := SSHArgs(h, nil)
	want := []string{
		"-N",
		"-i", "/tmp/id_ed25519",
		"-o", "CertificateFile=/tmp/id_ed25519-cert.pub",
		"-A",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		"example.com",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SSHArgs() =\n%q\nwant\n%q", got, want)
	}
}

func TestSSHArgsConnectionOptions(t *testing.T) {
	h := Host{
		HostName:        "example.com",
		User:            "filip",
		LogLevel:        "DEBUG1",
		BindAddress:     "192.168.1.10",
		AddressFamily:   "inet",
		ForwardAgent:    true,
		CertificateFile: "/tmp/id_ed25519-cert.pub",
		IdentityFile:    "/tmp/id_ed25519",
		NoSession:       true,
		Compression:     true,
		ExtraOptions: map[string]string{
			"ConnectTimeout":        "10",
			"IdentitiesOnly":        "yes",
			"ExitOnForwardFailure":  "no",
			"LogLevel":              "ERROR", // reserved — must not duplicate
			"ForwardAgent":          "no",   // reserved — dedicated -A wins
		},
	}
	tunnels := []Tunnel{{Type: TunnelDynamic, LocalPort: 1080}}
	got := SSHArgs(h, tunnels)
	want := []string{
		"-N",
		"-i", "/tmp/id_ed25519",
		"-o", "CertificateFile=/tmp/id_ed25519-cert.pub",
		"-C",
		"-A",
		"-b", "192.168.1.10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		"-o", "AddressFamily=inet",
		"-o", "LogLevel=DEBUG1",
		"-o", "ExitOnForwardFailure=no",
		"-o", "ConnectTimeout=10",
		"-o", "IdentitiesOnly=yes",
		"-D", "1080",
		"filip@example.com",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SSHArgs() =\n%q\nwant\n%q", got, want)
	}
}

func TestSSHArgsExitOnForwardFailureDefaultYes(t *testing.T) {
	got := SSHArgs(Host{HostName: "h", NoSession: true}, []Tunnel{{Type: TunnelDynamic, LocalPort: 1}})
	if !slices.Contains(got, "ExitOnForwardFailure=yes") {
		t.Fatalf("expected default ExitOnForwardFailure=yes, got %q", got)
	}
}

func TestValidateRequiresHostName(t *testing.T) {
	if err := Validate(Host{}, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTunnel(t *testing.T) {
	h := Host{HostName: "example.com"}
	if err := Validate(h, []Tunnel{{Type: TunnelLocal, LocalPort: 8080}}); err == nil {
		t.Fatal("expected error for missing remote")
	}
}
