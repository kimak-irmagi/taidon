//go:build !windows

package prepare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldRetryLockAcquireUnixBusyRegularFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "state.lock")
	if err := os.WriteFile(lockPath, []byte("lock"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	if !shouldRetryLockAcquire(os.ErrPermission, lockPath) {
		t.Fatalf("expected retry for busy regular lock file")
	}
}

func TestShouldRetryLockAcquireUnixStatErrorReturnsFalse(t *testing.T) {
	lockErr := &os.PathError{Op: "open", Path: "lock", Err: os.ErrExist}
	if shouldRetryLockAcquire(lockErr, "bad\x00path") {
		t.Fatalf("expected no retry when stat fails with non-permission error on unix")
	}
}

func TestIsLockBusyErrorUnixStatErrorReturnsFalse(t *testing.T) {
	if isLockBusyError(os.ErrPermission, "bad\x00path") {
		t.Fatalf("expected lock not busy when stat fails with non-permission error on unix")
	}
}
