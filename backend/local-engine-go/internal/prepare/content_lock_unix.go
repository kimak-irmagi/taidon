//go:build !windows

package prepare

import (
	"os"
	"syscall"
)

func lockFileShared(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
}

func unlockFileShared(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
