package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var runMountCommandFn = runMountCommand

func ensureStateStoreMount() error {
	unit := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_UNIT"))
	fstype := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_FSTYPE"))
	storeRoot := strings.TrimSpace(os.Getenv("SQLRS_STATE_STORE"))
	if unit == "" && fstype == "" {
		return nil
	}
	if unit == "" {
		return fmt.Errorf("SQLRS_WSL_MOUNT_UNIT must be set")
	}
	if storeRoot == "" {
		return fmt.Errorf("SQLRS_STATE_STORE is required to mount WSL device")
	}
	if fstype == "" {
		fstype = "btrfs"
	}
	if err := os.MkdirAll(storeRoot, 0o700); err != nil {
		return err
	}
	active, err := isSystemdUnitActive(unit)
	if err != nil {
		return fmt.Errorf("mount unit check failed: %w", err)
	}
	if !active {
		if _, err := runMountCommandFn("systemctl", "start", "--no-block", unit); err != nil {
			return appendMountLogs(unit, fmt.Errorf("mount unit start failed: %w", err))
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			active, err = isSystemdUnitActive(unit)
			if err != nil {
				return fmt.Errorf("mount unit check failed: %w", err)
			}
			if active {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if !active {
			return appendMountLogs(unit, fmt.Errorf("mount unit is not active"))
		}
	}
	fsType, mounted, err := waitForMountFSType(storeRoot, fstype)
	if err != nil {
		return err
	}
	if !mounted || fsType == "" {
		return fmt.Errorf("mount verification failed for %s", storeRoot)
	}
	if fsType != fstype {
		return appendMountLogs(unit, fmt.Errorf("mounted filesystem is %s, expected %s", fsType, fstype))
	}
	return nil
}

func findmntFSType(target string) (string, bool, error) {
	out, err := runMountCommandInInitNamespace("findmnt", "-n", "-o", "FSTYPE", "-T", target)
	if err == nil {
		return strings.TrimSpace(out), true, nil
	}
	if isExitStatus(err, 1) {
		return "", false, nil
	}
	return "", false, err
}

func waitForMountFSType(target, fstype string) (string, bool, error) {
	var lastType string
	var mounted bool
	for i := 0; i < 5; i++ {
		fsType, isMounted, err := findmntFSType(target)
		if err != nil {
			return "", false, err
		}
		lastType = fsType
		mounted = isMounted
		if mounted && fsType == fstype {
			return fsType, mounted, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastType, mounted, nil
}

func runMountCommandInInitNamespace(name string, args ...string) (string, error) {
	cmdArgs := append([]string{"-t", "1", "-m", "--", name}, args...)
	out, err := runMountCommandFn("nsenter", cmdArgs...)
	if err == nil {
		return out, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "command not found") {
		return runMountCommandFn(name, args...)
	}
	return out, err
}

func appendMountLogs(unit string, err error) error {
	tail, tailErr := runMountCommandFn("journalctl", "-u", unit, "-n", "20", "--no-pager")
	if tailErr == nil && strings.TrimSpace(tail) != "" {
		return fmt.Errorf("%v\n%s", err, strings.TrimSpace(tail))
	}
	return err
}

func isExitStatus(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

func isSystemdUnitActive(unit string) (bool, error) {
	out, err := runMountCommandFn("systemctl", "is-active", unit)
	if err != nil {
		if isExitStatus(err, 3) || isExitStatus(err, 4) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "active", nil
}

func runMountCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return string(output), fmt.Errorf("%w: %s", err, trimmed)
		}
		return string(output), err
	}
	return string(output), nil
}
