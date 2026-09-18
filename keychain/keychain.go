// Package keychain stores SSH secrets in the macOS Keychain.
// Tunnel profile JSON never contains passwords or passphrases.
package keychain

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "com.stenstromen.switchboard"

// Kind matches ssh.SecretKind values ("password" | "passphrase").
type Kind string

const (
	KindPassword   Kind = "password"
	KindPassphrase Kind = "passphrase"
)

func account(tunnelID string, kind Kind) string {
	return tunnelID + ":" + string(kind)
}

// Set stores a secret for the tunnel. Empty secret deletes the item.
func Set(tunnelID string, kind Kind, secret string) error {
	if tunnelID == "" {
		return fmt.Errorf("keychain: empty tunnel id")
	}
	if secret == "" {
		return Delete(tunnelID, kind)
	}
	return keyring.Set(service, account(tunnelID, kind), secret)
}

// Get returns a stored secret. Missing items return ("", nil).
func Get(tunnelID string, kind Kind) (string, error) {
	if tunnelID == "" {
		return "", nil
	}
	secret, err := keyring.Get(service, account(tunnelID, kind))
	if err == keyring.ErrNotFound {
		return "", nil
	}
	return secret, err
}

// Delete removes one secret. Missing items are ignored.
func Delete(tunnelID string, kind Kind) error {
	if tunnelID == "" {
		return nil
	}
	err := keyring.Delete(service, account(tunnelID, kind))
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}

// DeleteAll removes password and passphrase for a tunnel.
func DeleteAll(tunnelID string) error {
	err1 := Delete(tunnelID, KindPassword)
	err2 := Delete(tunnelID, KindPassphrase)
	if err1 != nil {
		return err1
	}
	return err2
}
