//go:build windows

package prepare

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockFileShared(file *os.File) error {
	var ol windows.Overlapped
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	return windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, 1, 0, &ol)
}

func unlockFileShared(file *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &ol)
}
