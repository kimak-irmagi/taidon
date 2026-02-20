package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureStateStoreMountDefaultsFSTypeAndWaitsForActivation(t *testing.T) {
	prev := runMountCommandFn
	call := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		call++
		switch call {
		case 1:
			return "inactive\n", exitError(3)
		case 2:
			return "", nil
		case 3:
			return "inactive\n", exitError(3)
		case 4:
			return "active\n", nil
		case 5:
			return "btrfs\n", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err != nil {
		t.Fatalf("ensureStateStoreMount: %v", err)
	}
}

func TestEnsureStateStoreMountReturnsMkdirError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", filepath.Join(parentFile, "store"))
	if err := ensureStateStoreMount(); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestEnsureStateStoreMountReturnsInitialUnitCheckError(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil || !strings.Contains(err.Error(), "mount unit check failed") {
		t.Fatalf("expected mount unit check error, got %v", err)
	}
}

func TestEnsureStateStoreMountReturnsPollingUnitCheckError(t *testing.T) {
	prev := runMountCommandFn
	call := 0
	runMountCommandFn = func(name string, args ...string) (string, error) {
		call++
		switch call {
		case 1:
			return "inactive\n", exitError(3)
		case 2:
			return "", nil
		default:
			return "", errors.New("boom")
		}
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	t.Setenv("SQLRS_WSL_MOUNT_UNIT", "sqlrs-state-store.mount")
	t.Setenv("SQLRS_WSL_MOUNT_FSTYPE", "btrfs")
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	if err := ensureStateStoreMount(); err == nil || !strings.Contains(err.Error(), "mount unit check failed") {
		t.Fatalf("expected polling unit check error, got %v", err)
	}
}

func TestRunMountCommandInInitNamespaceFallsBackWithoutNsenter(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		if name == "nsenter" {
			return "", errors.New("command not found")
		}
		if name == "findmnt" {
			return "btrfs\n", nil
		}
		return "", nil
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	out, err := runMountCommandInInitNamespace("findmnt", "-n", "-o", "FSTYPE", "-T", "/tmp")
	if err != nil {
		t.Fatalf("runMountCommandInInitNamespace: %v", err)
	}
	if strings.TrimSpace(out) != "btrfs" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestAppendMountLogsReturnsOriginalWhenNoJournalTail(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		return "", errors.New("journal unavailable")
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	rootErr := errors.New("root failure")
	err := appendMountLogs("sqlrs-state-store.mount", rootErr)
	if err == nil || err.Error() != rootErr.Error() {
		t.Fatalf("expected original error, got %v", err)
	}
}

func TestIsSystemdUnitActiveReturnsErrorForUnexpectedFailure(t *testing.T) {
	prev := runMountCommandFn
	runMountCommandFn = func(name string, args ...string) (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { runMountCommandFn = prev })

	active, err := isSystemdUnitActive("sqlrs-state-store.mount")
	if err == nil || active {
		t.Fatalf("expected unexpected failure error, active=%v err=%v", active, err)
	}
}

func TestRunMountCommandReturnsWrappedErrorWithOutput(t *testing.T) {
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "echo boom & exit 1"}
	} else {
		name = "sh"
		args = []string{"-c", "echo boom; exit 1"}
	}
	out, err := runMountCommand(name, args...)
	if err == nil {
		t.Fatalf("expected command error")
	}
	if strings.TrimSpace(out) == "" || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped output error, out=%q err=%v", out, err)
	}
}

func TestRunMountCommandReturnsRawErrorWithoutOutput(t *testing.T) {
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/c", "exit 1"}
	} else {
		name = "sh"
		args = []string{"-c", "exit 1"}
	}
	out, err := runMountCommand(name, args...)
	if err == nil {
		t.Fatalf("expected command error")
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}
