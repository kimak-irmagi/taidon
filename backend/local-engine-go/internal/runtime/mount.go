package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var runMountCommandFn = runMountCommand

func ensureStateStoreMount() error {
	device := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_DEVICE"))
	fstype := strings.TrimSpace(os.Getenv("SQLRS_WSL_MOUNT_FSTYPE"))
	storeRoot := strings.TrimSpace(os.Getenv("SQLRS_STATE_STORE"))
	if device == "" && fstype == "" {
		return nil
	}
	if device == "" || fstype == "" {
		return fmt.Errorf("SQLRS_WSL_MOUNT_DEVICE and SQLRS_WSL_MOUNT_FSTYPE must be set")
	}
	if storeRoot == "" {
		return fmt.Errorf("SQLRS_STATE_STORE is required to mount WSL device")
	}
	if err := os.MkdirAll(storeRoot, 0o700); err != nil {
		return err
	}
	fsType, mounted, err := findmntFSType(storeRoot)
	if err != nil {
		return err
	}
	if mounted {
		if fsType == "" {
			return fmt.Errorf("mount verification failed for %s", storeRoot)
		}
		if fsType != fstype {
			return fmt.Errorf("mounted filesystem is %s, expected %s", fsType, fstype)
		}
		return nil
	}
	if _, err := runMountCommandFn("mount", "-t", fstype, device, storeRoot); err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}
	fsType, mounted, err = findmntFSType(storeRoot)
	if err != nil {
		return err
	}
	if !mounted {
		return fmt.Errorf("mount verification failed for %s", storeRoot)
	}
	if fsType != fstype {
		return fmt.Errorf("mounted filesystem is %s, expected %s", fsType, fstype)
	}
	return nil
}

func findmntFSType(target string) (string, bool, error) {
	out, err := runMountCommandFn("findmnt", "-n", "-o", "FSTYPE", "-T", target)
	if err == nil {
		return strings.TrimSpace(out), true, nil
	}
	if isExitStatus(err, 1) {
		return "", false, nil
	}
	return "", false, err
}

func isExitStatus(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
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
