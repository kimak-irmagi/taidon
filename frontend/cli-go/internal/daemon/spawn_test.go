package daemon

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildDaemonCommand(t *testing.T) {
	runDir := filepath.Join("C:\\", "sqlrs", "run")
	statePath := filepath.Join("C:\\", "sqlrs", "engine.json")
	cmd, err := buildDaemonCommand("sqlrs-engine", runDir, statePath, "", "", "", "", "")
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
	cmd, err := buildDaemonCommand("/mnt/c/sqlrs/sqlrs-engine", runDir, statePath, "Ubuntu", "/var/lib/sqlrs/store", "/dev/sda2", "btrfs", filepath.Join("C:\\", "sqlrs", "logs", "engine.log"))
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
		if cmd.Args[1] != "-d" || cmd.Args[2] != "Ubuntu" {
			t.Fatalf("expected WSL distro args, got %+v", cmd.Args)
		}
	}
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
	if runtime.GOOS == "windows" {
		if !containsArg(cmd.Args, "SQLRS_STATE_STORE=/var/lib/sqlrs/store") {
			t.Fatalf("expected SQLRS_STATE_STORE to be passed via args")
		}
		if !containsArg(cmd.Args, "SQLRS_WSL_MOUNT_DEVICE=/dev/sda2") {
			t.Fatalf("expected SQLRS_WSL_MOUNT_DEVICE to be passed via args")
		}
		if !containsArg(cmd.Args, "SQLRS_WSL_MOUNT_FSTYPE=btrfs") {
			t.Fatalf("expected SQLRS_WSL_MOUNT_FSTYPE to be passed via args")
		}
	} else {
		if cmd.Env == nil || !containsEnv(cmd.Env, "SQLRS_STATE_STORE=/var/lib/sqlrs/store") {
			t.Fatalf("expected SQLRS_STATE_STORE env")
		}
		if !containsEnv(cmd.Env, "SQLRS_WSL_MOUNT_DEVICE=/dev/sda2") {
			t.Fatalf("expected SQLRS_WSL_MOUNT_DEVICE env")
		}
		if !containsEnv(cmd.Env, "SQLRS_WSL_MOUNT_FSTYPE=btrfs") {
			t.Fatalf("expected SQLRS_WSL_MOUNT_FSTYPE env")
		}
	}
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
