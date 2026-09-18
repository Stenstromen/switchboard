package ssh

// Status is the connection state of a host session.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusError        Status = "error"
)

// Snapshot is a point-in-time view of a managed SSH process.
type Snapshot struct {
	Status Status
	Err    string
	PID    int
	Logs   string
}
