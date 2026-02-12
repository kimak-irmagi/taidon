package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLockOpenError(t *testing.T) {
	if _, err := AcquireLock("\x00", time.Millisecond); err == nil {
		t.Fatalf("expected open error")
	}
}

func TestAcquireLockTryLockError(t *testing.T) {
	prev := tryLockFn
	tryLockFn = func(*os.File) (bool, error) {
		return false, errors.New("lock failed")
	}
	t.Cleanup(func() { tryLockFn = prev })

	path := filepath.Join(t.TempDir(), "daemon.lock")
	if _, err := AcquireLock(path, time.Millisecond); err == nil {
		t.Fatalf("expected tryLock error")
	}
}

func TestFileLockReleaseUnlockError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := AcquireLock(path, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	prev := unlockFn
	unlockFn = func(*os.File) error {
		return errors.New("unlock failed")
	}
	t.Cleanup(func() { unlockFn = prev })

	if err := lock.Release(); err == nil {
		t.Fatalf("expected unlock error")
	}
}

func TestFileLockReleaseNil(t *testing.T) {
	var lock *FileLock
	if err := lock.Release(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFileLockReleaseCloseError(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "lock-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	lock := &FileLock{file: file}

	prev := unlockFn
	unlockFn = func(*os.File) error { return nil }
	t.Cleanup(func() { unlockFn = prev })

	if err := lock.Release(); err == nil {
		t.Fatalf("expected close error")
	}
}
