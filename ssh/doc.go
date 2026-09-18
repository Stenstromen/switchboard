// Package ssh is a process manager around OpenSSH.
//
// It does not implement the SSH protocol. Host configuration is turned into an
// argument slice by SSHArgs, executed with exec.Command("/usr/bin/ssh", args...),
// and supervised by Manager (start/stop/status plus reconnect with backoff).
//
// Passwords and key passphrases are collected through OpenSSH's SSH_ASKPASS
// helper, never by stuffing a secret into the argument slice. Manager.SetPassword
// caches a secret in memory for the session; the app layer may also persist it in
// the macOS Keychain. WithPrompter asks the UI when ssh needs one.
//
// A Host describes how to reach an SSH server. A Tunnel describes what that
// session should forward. The GUI-facing API is Manager.Connect(hostID);
// callers never need to handle exec.Cmd.
package ssh
