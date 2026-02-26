package prepare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLockBusyError(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, "lock")
	if err := os.WriteFile(lockPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if !isLockBusyError(os.ErrPermission, lockPath) {
		t.Fatalf("expected lock busy for permission error with existing lock")
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("remove lock: %v", err)
	}
	if isLockBusyError(os.ErrPermission, lockPath) {
		t.Fatalf("expected lock not busy when lock file is missing")
	}
	if isLockBusyError(os.ErrNotExist, lockPath) {
		t.Fatalf("expected lock not busy for non-permission error")
	}
	if isLockBusyError(nil, lockPath) {
		t.Fatalf("expected lock not busy for nil error")
	}
}

func TestShouldRetryLockAcquirePermissionWhenLockDisappeared(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "lock")
	lockErr := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrPermission}
	if !shouldRetryLockAcquire(lockErr, lockPath) {
		t.Fatalf("expected retry for transient permission error when lock file disappeared")
	}
}

func TestIsLockBusyErrorDirectoryPath(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "lock-dir")
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if isLockBusyError(os.ErrPermission, lockPath) {
		t.Fatalf("expected lock not busy when lock path points to directory")
	}
}
