//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func configureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func configureDetachedWSL(cmd *exec.Cmd) {
	configureDetached(cmd)
}
