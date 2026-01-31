package daemon

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func buildDaemonCommand(path, runDir, statePath, wslDistro, storeDir, mountUnit, mountFSType, logPath string) (*exec.Cmd, error) {
	args := []string{
		"--run-dir", runDir,
		"--listen", "127.0.0.1:0",
		"--write-engine-json", statePath,
	}
	if wslDistro != "" {
		wslCmd := []string{"-d", wslDistro, "-u", "root", "--"}
		envVars := []string{}
		if storeDir != "" {
			envVars = append(envVars, "SQLRS_STATE_STORE="+storeDir)
		}
		if mountUnit != "" {
			envVars = append(envVars, "SQLRS_WSL_MOUNT_UNIT="+mountUnit)
		}
		if mountFSType != "" {
			envVars = append(envVars, "SQLRS_WSL_MOUNT_FSTYPE="+mountFSType)
		}
		if len(envVars) > 0 {
			wslCmd = append(wslCmd, "env")
			wslCmd = append(wslCmd, envVars...)
		}
		wslCmd = append(wslCmd, "nsenter", "-t", "1", "-m", "--", path)
		wslCmd = append(wslCmd, args...)
		if runtime.GOOS == "windows" {
			cmdline := buildCmdLine("wsl.exe", wslCmd, "")
			appendLogLine(logPath, "spawn wsl: "+cmdline)
		}
		cmd := exec.Command("wsl.exe", wslCmd...)
		cmd.Stdin = nil
		if runtime.GOOS == "windows" {
			configureDetachedWSL(cmd)
		} else {
			configureDetached(cmd)
		}
		return cmd, nil
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = nil
	envVars := []string{}
	if storeDir != "" {
		envVars = append(envVars, "SQLRS_STATE_STORE="+storeDir)
	}
	if mountUnit != "" {
		envVars = append(envVars, "SQLRS_WSL_MOUNT_UNIT="+mountUnit)
	}
	if mountFSType != "" {
		envVars = append(envVars, "SQLRS_WSL_MOUNT_FSTYPE="+mountFSType)
	}
	if len(envVars) > 0 {
		cmd.Env = append(os.Environ(), envVars...)
	}
	configureDetached(cmd)
	return cmd, nil
}

func quotePowerShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func buildCmdLine(exe string, args []string, logPath string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteCmd(exe))
	for _, arg := range args {
		parts = append(parts, quoteCmd(arg))
	}
	cmdline := strings.Join(parts, " ")
	if strings.TrimSpace(logPath) != "" {
		cmdline += " >> " + quoteCmd(logPath) + " 2>&1"
	}
	return cmdline
}

func quoteCmd(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "\"\""
	}
	if strings.ContainsAny(value, " \t\"") {
		value = strings.ReplaceAll(value, "\"", "\\\"")
		return "\"" + value + "\""
	}
	return value
}

func appendLogLine(path string, line string) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(line) == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = file.WriteString(line + "\n")
	_ = file.Close()
}
