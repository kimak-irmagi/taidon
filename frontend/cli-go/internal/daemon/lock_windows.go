//go:build windows

package daemon

import (
	"os"

	"golang.org/x/sys/windows"
)

func tryLock(file *os.File) (bool, error) {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err == nil {
		return true, nil
	}
	if err == windows.ERROR_LOCK_VIOLATION {
		return false, nil
	}
	return false, err
}

func unlock(file *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &ol)
}
