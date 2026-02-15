package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureWSLMountRequiresUnitWhenOnlyFSTypeProvided(t *testing.T) {
	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(t.TempDir()); err == nil || !strings.Contains(err.Error(), "SQLRS_WSL_MOUNT_UNIT must be set") {
		t.Fatalf("expected unit-required error, got %v", err)
	}
}

func TestEnsureWSLMountUsesDefaultFSTypeWhenMissing(t *testing.T) {
	prevRun := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch calls {
		case 1:
			if name != "systemctl" {
				t.Fatalf("expected systemctl call, got %s", name)
			}
			return "active\n", nil
		case 2:
			if name != "findmnt" {
				t.Fatalf("expected findmnt call, got %s", name)
			}
			return "btrfs\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prevRun })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "")
	if err := ensureWSLMount(t.TempDir()); err != nil {
		t.Fatalf("ensureWSLMount: %v", err)
	}
}

func TestEnsureWSLMountStateStoreMkdirFailure(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "state-store")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(filePath); err == nil {
		t.Fatalf("expected mkdir failure")
	}
}

func TestEnsureWSLMountInitialStatusCheckError(t *testing.T) {
	prevRun := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "is-active" {
			return "", errors.New("boom")
		}
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prevRun })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(t.TempDir()); err == nil || !strings.Contains(err.Error(), "mount unit check failed") {
		t.Fatalf("expected mount check error, got %v", err)
	}
}

func TestEnsureWSLMountRecheckErrorAfterStart(t *testing.T) {
	prevRun := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch calls {
		case 1:
			return "inactive\n", exitError(3)
		case 2:
			return "", nil
		case 3:
			return "", errors.New("boom")
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prevRun })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(t.TempDir()); err == nil || !strings.Contains(err.Error(), "mount unit check failed") {
		t.Fatalf("expected recheck error, got %v", err)
	}
}

func TestEnsureWSLMountInactiveAfterStart(t *testing.T) {
	prevRun := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch calls {
		case 1:
			return "inactive\n", exitError(3)
		case 2:
			return "", nil
		case 3:
			return "inactive\n", exitError(3)
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prevRun })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(t.TempDir()); err == nil || !strings.Contains(err.Error(), "mount unit is not active") {
		t.Fatalf("expected inactive-after-start error, got %v", err)
	}
}

func TestEnsureWSLMountMissingFindmntEntry(t *testing.T) {
	prevRun := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		switch name {
		case "systemctl":
			return "active\n", nil
		case "findmnt":
			return "", exitError(1)
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prevRun })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	if err := ensureWSLMount(t.TempDir()); err == nil || !strings.Contains(err.Error(), "mount verification failed") {
		t.Fatalf("expected mount verification error, got %v", err)
	}
}

