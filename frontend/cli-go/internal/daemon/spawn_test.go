package daemon

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildDaemonCommand(t *testing.T) {
	runDir := filepath.Join("C:\\", "sqlrs", "run")
	statePath := filepath.Join("C:\\", "sqlrs", "engine.json")
	cmd, err := buildDaemonCommand("sqlrs-engine", runDir, statePath, "", "", "", "", 0, "")
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "sqlrs-engine" {
		t.Fatalf("unexpected args: %+v", cmd.Args)
	}
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
}

func TestBuildDaemonCommandWSL(t *testing.T) {
	runDir := "/var/lib/sqlrs/run"
	statePath := "/mnt/c/sqlrs/engine.json"
	cmd, err := buildDaemonCommand("/mnt/c/sqlrs/sqlrs-engine", runDir, statePath, "Ubuntu", "/var/lib/sqlrs/store", "sqlrs-state-store.mount", "btrfs", 0, filepath.Join("C:\\", "sqlrs", "logs", "engine.log"))
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if runtime.GOOS == "windows" {
		if len(cmd.Args) < 5 || cmd.Args[0] != "wsl.exe" {
			t.Fatalf("unexpected args: %+v", cmd.Args)
		}
	} else {
		if len(cmd.Args) < 5 || cmd.Args[0] != "wsl.exe" {
			t.Fatalf("unexpected args: %+v", cmd.Args)
		}
		if cmd.Args[1] != "-d" || cmd.Args[2] != "Ubuntu" || cmd.Args[3] != "-u" || cmd.Args[4] != "root" {
			t.Fatalf("expected WSL distro args, got %+v", cmd.Args)
		}
	}
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
	if !containsArg(cmd.Args, "SQLRS_STATE_STORE=/var/lib/sqlrs/store") {
		t.Fatalf("expected SQLRS_STATE_STORE to be passed via args")
	}
	if !containsArg(cmd.Args, "SQLRS_WSL_MOUNT_UNIT=sqlrs-state-store.mount") {
		t.Fatalf("expected SQLRS_WSL_MOUNT_UNIT to be passed via args")
	}
	if !containsArg(cmd.Args, "SQLRS_WSL_MOUNT_FSTYPE=btrfs") {
		t.Fatalf("expected SQLRS_WSL_MOUNT_FSTYPE to be passed via args")
	}
	if !containsArg(cmd.Args, "nsenter") {
		t.Fatalf("expected nsenter to be used in WSL command")
	}
}

func TestBuildDaemonCommandEnvVars(t *testing.T) {
	runDir := filepath.Join("C:\\", "sqlrs", "run")
	statePath := filepath.Join("C:\\", "sqlrs", "engine.json")
	cmd, err := buildDaemonCommand("sqlrs-engine", runDir, statePath, "", "C:\\store", "unit.mount", "btrfs", 0, "")
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if !containsEnv(cmd.Env, "SQLRS_STATE_STORE=C:\\store") {
		t.Fatalf("expected SQLRS_STATE_STORE in env")
	}
	if !containsEnv(cmd.Env, "SQLRS_WSL_MOUNT_UNIT=unit.mount") {
		t.Fatalf("expected SQLRS_WSL_MOUNT_UNIT in env")
	}
	if !containsEnv(cmd.Env, "SQLRS_WSL_MOUNT_FSTYPE=btrfs") {
		t.Fatalf("expected SQLRS_WSL_MOUNT_FSTYPE in env")
	}
}

func TestBuildDaemonCommandWSLNoEnv(t *testing.T) {
	runDir := "/var/lib/sqlrs/run"
	statePath := "/mnt/c/sqlrs/engine.json"
	cmd, err := buildDaemonCommand("/mnt/c/sqlrs/sqlrs-engine", runDir, statePath, "Ubuntu", "", "", "", 0, "")
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if containsArg(cmd.Args, "SQLRS_STATE_STORE=") {
		t.Fatalf("did not expect SQLRS_STATE_STORE in args")
	}
}

func TestQuotePowerShell(t *testing.T) {
	if got := quotePowerShell("O'Reilly"); got != "'O''Reilly'" {
		t.Fatalf("unexpected powershell quote: %s", got)
	}
}

func TestBuildDaemonCommandWithIdleTimeout(t *testing.T) {
	runDir := filepath.Join("C:\\", "sqlrs", "run")
	statePath := filepath.Join("C:\\", "sqlrs", "engine.json")
	cmd, err := buildDaemonCommand("sqlrs-engine", runDir, statePath, "", "", "", "", 2*time.Minute, "")
	if err != nil {
		t.Fatalf("buildDaemonCommand: %v", err)
	}
	if !containsArg(cmd.Args, "--idle-timeout") {
		t.Fatalf("expected --idle-timeout flag in args, got %+v", cmd.Args)
	}
	if !containsArg(cmd.Args, "2m0s") {
		t.Fatalf("expected idle timeout value in args, got %+v", cmd.Args)
	}
}

func TestBuildCmdLineWithLog(t *testing.T) {
	cmdline := buildCmdLine("wsl.exe", []string{"--arg", "value with space"}, "C:\\logs\\engine.log")
	if !strings.Contains(cmdline, ">>") || !strings.Contains(cmdline, "engine.log") {
		t.Fatalf("expected log redirect, got %s", cmdline)
	}
}

func TestQuoteCmdVariants(t *testing.T) {
	if quoteCmd("") != "\"\"" {
		t.Fatalf("expected empty quote")
	}
	if out := quoteCmd("has space"); !strings.HasPrefix(out, "\"") {
		t.Fatalf("expected quoted, got %s", out)
	}
	if out := quoteCmd(`has"quote`); !strings.Contains(out, "\\\"") {
		t.Fatalf("expected escaped quote, got %s", out)
	}
}

func TestAppendLogLineSkipsEmpty(t *testing.T) {
	appendLogLine("", "line")
	appendLogLine("path", "")
}

func TestAppendLogLineOpenError(t *testing.T) {
	dir := t.TempDir()
	appendLogLine(dir, "line")
}

func containsEnv(env []string, value string) bool {
	for _, item := range env {
		if item == value {
			return true
		}
	}
	return false
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if strings.Contains(arg, value) {
			return true
		}
	}
	return false
}
