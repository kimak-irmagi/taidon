//go:build windows

package app

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"sqlrs/cli/internal/wsl"
)

func TestInitWSLHappyPathWithStubs(t *testing.T) {
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		t.Skip("wsl.exe not available")
	}

	storeDir := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", storeDir)

	withListWSLDistrosStub(t, func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true, State: "Running", Version: 2}}, nil
	})
	withIsElevatedStub(t, func(verbose bool) (bool, error) {
		return true, nil
	})

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "starting WSL distro":
			return "", nil
		case "check btrfs kernel":
			return "nodev   btrfs\n", nil
		case "check btrfs-progs":
			return "/usr/sbin/mkfs.btrfs\n", nil
		case "check docker in WSL":
			return "OK", nil
		case "resolve XDG_STATE_HOME":
			return "/state", nil
		case "lsblk":
			return "NAME SIZE TYPE PKNAME\nsda 107374182400 disk\n├─sda1 107373182976 part sda\n", nil
		case "detect filesystem":
			return "btrfs\n", nil
		case "findmnt (root)":
			return "", errExitStatus1
		case "create state dir":
			return "", nil
		case "check mountpoint":
			return "", nil
		case "check path":
			return "", errExitStatus1
		case "create subvolume (root)":
			return "", nil
		case "resolve WSL user":
			return "user\n", nil
		case "resolve WSL group":
			return "group\n", nil
		case "chown btrfs (root)":
			return "", nil
		default:
			return "", errUnexpected(desc)
		}
	})

	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "Running\n", nil
		case "create VHDX", "partition VHDX", "attach VHDX to WSL":
			return "", nil
		default:
			return "", errUnexpected(desc)
		}
	})

	result, err := initWSL(wslInitOptions{
		Enable:      true,
		Distro:      "",
		Require:     true,
		NoStart:     false,
		Workspace:   t.TempDir(),
		Verbose:     false,
		StoreSizeGB: 100,
	})
	if err != nil {
		t.Fatalf("initWSL: %v", err)
	}
	if !result.UseWSL {
		t.Fatalf("expected UseWSL true")
	}
	if result.Distro != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %q", result.Distro)
	}
	if result.StateDir != "/state/sqlrs/store" {
		t.Fatalf("expected state dir, got %q", result.StateDir)
	}
	expectedStorePath := filepath.Join(storeDir, defaultVHDXName)
	if result.StorePath != expectedStorePath {
		t.Fatalf("expected store path %q, got %q", expectedStorePath, result.StorePath)
	}
	if result.MountDevice != "/dev/sda1" {
		t.Fatalf("expected mount device /dev/sda1, got %q", result.MountDevice)
	}
	if result.MountFSType != "btrfs" {
		t.Fatalf("expected mount fstype btrfs, got %q", result.MountFSType)
	}
}

func withListWSLDistrosStub(t *testing.T, fn func() ([]wsl.Distro, error)) {
	t.Helper()
	prev := listWSLDistrosFn
	listWSLDistrosFn = fn
	t.Cleanup(func() {
		listWSLDistrosFn = prev
	})
}

var errExitStatus1 = errInitTest("exit status 1")

func errUnexpected(desc string) error {
	return errInitTest("unexpected " + desc)
}
