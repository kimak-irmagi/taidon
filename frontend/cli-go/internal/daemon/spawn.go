package daemon

import "os/exec"

func buildDaemonCommand(path, runDir, statePath string) (*exec.Cmd, error) {
	args := []string{
		"--run-dir", runDir,
		"--listen", "127.0.0.1:0",
		"--write-engine-json", statePath,
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = nil
	configureDetached(cmd)
	return cmd, nil
}
