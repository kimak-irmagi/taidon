//go:build windows

package prepare

import (
	"os"
	"testing"
)

func TestIsLockBusyErrorWindowsPermissionStatError(t *testing.T) {
	lockPath := windowsPermissionDeniedPath(t)
	if !isLockBusyError(os.ErrPermission, lockPath) {
		t.Fatalf("expected lock busy when stat returns permission error on windows")
	}
}

func TestShouldRetryLockAcquireWindowsPermissionStatError(t *testing.T) {
	lockPath := windowsPermissionDeniedPath(t)
	lockErr := &os.PathError{Op: "open", Path: lockPath, Err: os.ErrExist}
	if !shouldRetryLockAcquire(lockErr, lockPath) {
		t.Fatalf("expected retry when lock stat returns permission error on windows")
	}
}

func windowsPermissionDeniedPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		`C:\System Volume Information`,
		`C:\$Recycle.Bin`,
	}
	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err != nil && os.IsPermission(err) {
			return candidate
		}
	}
	t.Skip("no permission-denied path available on this host")
	return ""
}
