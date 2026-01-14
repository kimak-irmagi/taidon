package daemon

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockTimeout(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "daemon.lock")

	lock, err := AcquireLock(path, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer lock.Release()

	_, err = AcquireLock(path, 100*time.Millisecond)
	if !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("expected lock timeout, got %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	lock2, err := AcquireLock(path, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("reacquire lock: %v", err)
	}
	lock2.Release()
}
