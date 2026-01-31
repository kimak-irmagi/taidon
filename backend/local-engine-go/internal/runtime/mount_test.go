package runtime

import (
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestEnsureStateStoreMountSkippedWithoutEnv(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(string, ...string) (string, error) {
		t.Fatalf("unexpected mount command")
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "")
	t.Setenv("SQLRS_STATE_STORE", "")
	if err := ensureStateStoreMount(); err != nil {
		t.Fatalf("ensureStateStoreMount: %v", err)
	}
}

func TestEnsureStateStoreMountRequiresUnit(t *testing.T) {
	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", "/tmp/sqlrs")
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected missing unit error")
	}
}

func TestEnsureStateStoreMountRequiresStoreRoot(t *testing.T) {
	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", "")
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected missing store error")
	}
}

func TestEnsureStateStoreMountAlreadyActive(t *testing.T) {
	prev := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch calls {
		case 1:
			if name != "systemctl" {
				t.Fatalf("expected systemctl, got %s", name)
			}
			return "active\n", nil
		case 2:
			if name != "nsenter" {
				t.Fatalf("expected nsenter, got %s", name)
			}
			return "btrfs\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err != nil {
		t.Fatalf("ensureStateStoreMount: %v", err)
	}
}

func TestEnsureStateStoreMountStartsUnit(t *testing.T) {
	prev := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch calls {
		case 1:
			return "inactive\n", exitError(3)
		case 2:
			if name != "systemctl" || args[0] != "start" || args[1] != "--no-block" {
				t.Fatalf("expected systemctl start --no-block, got %s %v", name, args)
			}
			return "", nil
		case 3:
			return "active\n", nil
		case 4:
			if name != "nsenter" {
				t.Fatalf("expected nsenter, got %s", name)
			}
			return "btrfs\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err != nil {
		t.Fatalf("ensureStateStoreMount: %v", err)
	}
}

func TestEnsureStateStoreMountStartFails(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "is-active" {
			return "inactive\n", exitError(3)
		}
		if name == "systemctl" && len(args) > 1 && args[0] == "start" {
			return "boom\n", errors.New("fail")
		}
		if name == "journalctl" {
			return "unit failed\n", nil
		}
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected start error")
	}
}

func TestEnsureStateStoreMountWrongFS(t *testing.T) {
	prev := runMountCommandFn
	calls := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		calls++
		switch name {
		case "systemctl":
			return "active\n", nil
		case "nsenter":
			if calls < 3 {
				return "ext4\n", nil
			}
			return "ext4\n", nil
		case "journalctl":
			return "unit failed\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected fstype error")
	}
}

func TestEnsureStateStoreMountFindmntError(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		if name == "systemctl" {
			return "active\n", nil
		}
		if name == "nsenter" {
			return "", errors.New("boom")
		}
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected findmnt error")
	}
}

func TestEnsureStateStoreMountNotMounted(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		if name == "systemctl" {
			return "active\n", nil
		}
		if name == "nsenter" {
			return "", exitError(1)
		}
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected mount verification error")
	}
}

func exitError(code int) error {
	cmd := exec.Command("sh", "-c", "exit "+strconv.Itoa(code))
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit "+strconv.Itoa(code))
	}
	if err := cmd.Run(); err == nil {
		return errors.New("expected exit error")
	} else {
		return err
	}
}

func TestRunMountCommandOutput(t *testing.T) {
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "echo", "ok"}
	} else {
		name = "sh"
		args = []string{"-c", "echo ok"}
	}
	out, err := runMountCommand(name, args...)
	if err != nil {
		t.Fatalf("runMountCommand: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("expected output ok, got %q", out)
	}
}
