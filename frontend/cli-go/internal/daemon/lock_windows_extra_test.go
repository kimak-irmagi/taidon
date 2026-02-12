//go:build windows

package daemon

import (
	"os"
	"testing"
)

func TestTryLockError(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "lock-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	ok, err := tryLock(file)
	if err == nil || ok {
		t.Fatalf("expected tryLock error, ok=%v err=%v", ok, err)
	}
}
