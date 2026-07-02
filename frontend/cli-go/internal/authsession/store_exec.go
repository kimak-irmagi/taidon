//go:build linux

package authsession

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// SystemCredentialStore uses Secret Service/libsecret on Linux. It deliberately
// has no plaintext fallback.
type SystemCredentialStore struct{}

func NewSystemCredentialStore() CredentialStore {
	return SystemCredentialStore{}
}

func (SystemCredentialStore) Get(ctx context.Context, key CredentialKey) (Session, bool, error) {
	out, err := exec.CommandContext(ctx, "secret-tool", "lookup", "application", credentialService, "key", key.displayAccount()).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return Session{}, false, fmt.Errorf("Linux Secret Service lookup failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return Session{}, false, nil
	}
	session, err := decodeSession(strings.TrimSpace(string(out)))
	if err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (SystemCredentialStore) Put(ctx context.Context, key CredentialKey, session Session) error {
	encoded := encodeSession(session)
	cmd := exec.CommandContext(ctx, "secret-tool", "store", "--label", "sqlrs auth session", "application", credentialService, "key", key.displayAccount())
	cmd.Stdin = strings.NewReader(encoded)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Linux Secret Service store failed; install/start a Secret Service provider such as GNOME Keyring or KWallet: %w", err)
	}
	return nil
}

func (SystemCredentialStore) Delete(ctx context.Context, key CredentialKey) error {
	_ = exec.CommandContext(ctx, "secret-tool", "clear", "application", credentialService, "key", key.displayAccount()).Run()
	return nil
}
