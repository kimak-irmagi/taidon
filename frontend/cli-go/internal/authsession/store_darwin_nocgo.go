//go:build darwin && !cgo

package authsession

import (
	"context"
	"fmt"
)

// SystemCredentialStore requires cgo on macOS so the CLI can call Keychain
// Services without passing refresh-token session data through process args.
type SystemCredentialStore struct{}

func NewSystemCredentialStore() CredentialStore {
	return SystemCredentialStore{}
}

func (SystemCredentialStore) Get(context.Context, CredentialKey) (Session, bool, error) {
	return Session{}, false, darwinNoCGOError()
}

func (SystemCredentialStore) Put(context.Context, CredentialKey, Session) error {
	return darwinNoCGOError()
}

func (SystemCredentialStore) Delete(context.Context, CredentialKey) error {
	return darwinNoCGOError()
}

func darwinNoCGOError() error {
	return fmt.Errorf("macOS Keychain credential store requires cgo")
}
