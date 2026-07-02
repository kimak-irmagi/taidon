//go:build windows

package authsession

import (
	"context"
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestSystemCredentialStoreWindowsPutGetDelete(t *testing.T) {
	key := testCredentialKey()
	var stored []byte
	var freed bool
	var deleted bool
	restoreWindowsCredentialFns(t)

	windowsCredWrite = func(cred *winCredential) error {
		if cred.Type != credTypeGeneric || cred.Persist != credPersistLocalMachine {
			t.Fatalf("credential metadata = type %d persist %d", cred.Type, cred.Persist)
		}
		stored = append([]byte(nil), unsafe.Slice(cred.CredentialBlob, cred.CredentialBlobSize)...)
		return nil
	}
	windowsCredRead = func(_ *uint16, typ uint32, flags uint32, out **winCredential) error {
		if typ != credTypeGeneric || flags != 0 {
			t.Fatalf("read typ=%d flags=%d", typ, flags)
		}
		if len(stored) == 0 {
			return windows.ERROR_NOT_FOUND
		}
		*out = &winCredential{CredentialBlobSize: uint32(len(stored)), CredentialBlob: &stored[0]}
		return nil
	}
	windowsCredFree = func(*winCredential) {
		freed = true
	}
	windowsCredDelete = func(_ *uint16, typ uint32, flags uint32) error {
		if typ != credTypeGeneric || flags != 0 {
			t.Fatalf("delete typ=%d flags=%d", typ, flags)
		}
		deleted = true
		return nil
	}

	store := SystemCredentialStore{}
	session := Session{Provider: "google", RefreshToken: "refresh-token", CachedIDToken: "id-token"}
	if err := store.Put(context.Background(), key, session); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := store.Get(context.Background(), key)
	if err != nil || !ok {
		t.Fatalf("Get ok=%v err=%v", ok, err)
	}
	if got.RefreshToken != "refresh-token" || got.CachedIDToken != "id-token" {
		t.Fatalf("session = %+v", got)
	}
	if !freed {
		t.Fatalf("expected credential memory to be freed")
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatalf("expected delete call")
	}
}

func TestSystemCredentialStoreWindowsErrors(t *testing.T) {
	key := testCredentialKey()
	store := SystemCredentialStore{}

	t.Run("missing read", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredRead = func(*uint16, uint32, uint32, **winCredential) error {
			return windows.ERROR_NOT_FOUND
		}
		_, ok, err := store.Get(context.Background(), key)
		if err != nil || ok {
			t.Fatalf("missing read ok=%v err=%v", ok, err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredRead = func(*uint16, uint32, uint32, **winCredential) error {
			return errors.New("read failed")
		}
		_, _, err := store.Get(context.Background(), key)
		if err == nil || err.Error() != "read failed" {
			t.Fatalf("expected read failed, got %v", err)
		}
	})

	t.Run("invalid stored session", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		blob := []byte("{")
		windowsCredRead = func(_ *uint16, _ uint32, _ uint32, out **winCredential) error {
			*out = &winCredential{CredentialBlobSize: uint32(len(blob)), CredentialBlob: &blob[0]}
			return nil
		}
		windowsCredFree = func(*winCredential) {}
		if _, _, err := store.Get(context.Background(), key); err == nil {
			t.Fatalf("expected decode error")
		}
	})

	t.Run("empty blob", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredRead = func(_ *uint16, _ uint32, _ uint32, out **winCredential) error {
			*out = &winCredential{}
			return nil
		}
		windowsCredFree = func(*winCredential) {}
		_, ok, err := store.Get(context.Background(), key)
		if err != nil || ok {
			t.Fatalf("empty blob ok=%v err=%v", ok, err)
		}
	})

	t.Run("write error", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredWrite = func(*winCredential) error {
			return errors.New("write failed")
		}
		if err := store.Put(context.Background(), key, Session{RefreshToken: "refresh"}); err == nil || err.Error() != "write failed" {
			t.Fatalf("expected write failed, got %v", err)
		}
	})

	t.Run("delete missing ignored", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredDelete = func(*uint16, uint32, uint32) error {
			return windows.ERROR_NOT_FOUND
		}
		if err := store.Delete(context.Background(), key); err != nil {
			t.Fatalf("missing delete should be ignored: %v", err)
		}
	})

	t.Run("delete error", func(t *testing.T) {
		restoreWindowsCredentialFns(t)
		windowsCredDelete = func(*uint16, uint32, uint32) error {
			return errors.New("delete failed")
		}
		if err := store.Delete(context.Background(), key); err == nil || err.Error() != "delete failed" {
			t.Fatalf("expected delete failed, got %v", err)
		}
	})
}

func TestWindowsCredentialWrappersRejectInvalidOrMissingCredentials(t *testing.T) {
	target, err := windows.UTF16PtrFromString("sqlrs-cli-auth:unit-test-nonexistent-credential")
	if err != nil {
		t.Fatalf("UTF16 target: %v", err)
	}

	var cred *winCredential
	if err := windowsCredRead(target, credTypeGeneric, 0, &cred); !errors.Is(err, windows.ERROR_NOT_FOUND) {
		t.Fatalf("read nonexistent credential = %v, want ERROR_NOT_FOUND", err)
	}
	if err := windowsCredDelete(target, credTypeGeneric, 0); !errors.Is(err, windows.ERROR_NOT_FOUND) {
		t.Fatalf("delete nonexistent credential = %v, want ERROR_NOT_FOUND", err)
	}
	if err := windowsCredWrite(&winCredential{Type: credTypeGeneric, Persist: credPersistLocalMachine}); err == nil {
		t.Fatalf("expected invalid credential write to fail")
	}
}

func restoreWindowsCredentialFns(t *testing.T) {
	t.Helper()
	oldRead := windowsCredRead
	oldWrite := windowsCredWrite
	oldDelete := windowsCredDelete
	oldFree := windowsCredFree
	t.Cleanup(func() {
		windowsCredRead = oldRead
		windowsCredWrite = oldWrite
		windowsCredDelete = oldDelete
		windowsCredFree = oldFree
	})
}
