package ssh

// Host describes how to reach an SSH server. It is not a forwarding rule;
// attach those separately with Tunnel.
type Host struct {
	// Name is the local identifier used by Manager.Connect.
	Name string

	// HostName is the SSH server address (OpenSSH HostName).
	HostName string

	User string

	// Port is passed as -p when non-zero. Zero leaves OpenSSH's default (22).
	Port int

	ProxyJump string

	// StrictHostKeyChecking is the OpenSSH option value, e.g. "accept-new".
	StrictHostKeyChecking string

	HashKnownHosts bool

	IdentityFile string

	// CertificateFile is an OpenSSH certificate (CertificateFile) used with IdentityFile.
	CertificateFile string

	// ServerAliveInterval and ServerAliveCountMax map to the OpenSSH keepalive
	// options. Zero omits them.
	ServerAliveInterval int
	ServerAliveCountMax int

	// NoSession requests -N (no remote command). Tunnel managers usually want this.
	// When false, -N is omitted.
	NoSession bool

	Compression   bool
	BindAddress   string
	AddressFamily string // any | inet | inet6
	ForwardAgent  bool
	LogLevel      string

	// MaxRetries is how many consecutive failed connect attempts are allowed
	// before the watchdog gives up. Zero means try once (no retries).
	// Negative means unlimited.
	MaxRetries int

	// ExtraOptions are arbitrary OpenSSH -o Key=Value pairs from the Advanced tab.
	// Keys already expressed by dedicated fields should generally be omitted here.
	ExtraOptions map[string]string
}

func (h Host) id() string {
	if h.Name != "" {
		return h.Name
	}
	return h.HostName
}

func (h Host) destination() string {
	if h.User != "" {
		return h.User + "@" + h.HostName
	}
	return h.HostName
}
