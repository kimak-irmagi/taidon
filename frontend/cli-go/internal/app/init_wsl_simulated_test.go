package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sqlrs/cli/internal/wsl"
)

func withWindowsMode(t *testing.T) {
	t.Helper()
	prev := isWindows
	isWindows = true
	t.Cleanup(func() { isWindows = prev })
}

func withFakeWslExe(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "wsl.exe")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write wsl.exe: %v", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(exe, 0o700)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInitWSLUnavailableNoExeSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	t.Setenv("PATH", "")

	res, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got err=%v", err)
	}
	if res.UseWSL || res.Warning == "" {
		t.Fatalf("expected warning result, got %+v", res)
	}
}

func TestInitWSLUnavailableRequireSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	t.Setenv("PATH", "")

	_, err := initWSL(wslInitOptions{Enable: true, Require: true})
	if err == nil || !strings.Contains(err.Error(), "WSL is not available") {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestInitWSLNotElevatedSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true}}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "starting WSL distro":
			return "", nil
		case "check btrfs kernel":
			return "nodev btrfs\n", nil
		case "check btrfs-progs":
			return "/usr/sbin/mkfs.btrfs\n", nil
		case "check nsenter":
			return "/usr/bin/nsenter\n", nil
		default:
			return "", nil
		}
	})
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "running\n", nil
	})
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "False\n", nil
		case "check docker cli":
			return "", errors.New("not running")
		default:
			return "", nil
		}
	})
	withIsElevatedStub(t, func(bool) (bool, error) { return false, nil })

	res, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || res.Warning == "" {
		t.Fatalf("expected warning result, got %+v", res)
	}
}

func TestInitWSLSuccessSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true}}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "starting WSL distro":
			return "", nil
		case "check btrfs kernel":
			return "nodev btrfs\n", nil
		case "check btrfs-progs":
			return "/usr/sbin/mkfs.btrfs\n", nil
		case "check nsenter":
			return "/usr/bin/nsenter\n", nil
		case "resolve XDG_STATE_HOME":
			return "/state\n", nil
		case "resolve mount unit (root)":
			return "sqlrs-state-store.mount\n", nil
		case "lsblk":
			return "NAME SIZE TYPE PKNAME\nsda 10737418240 disk\n└─sda1 10737318240 part sda\n", nil
		case "findmnt (root)":
			return "btrfs\n", nil
		case "detect filesystem":
			return "\n", nil
		case "wipefs (root)":
			return "", errInitTest("command not found")
		case "format btrfs (root)":
			return "", nil
		case "verify filesystem (root)":
			return "btrfs\n", nil
		case "resolve partition UUID (root)":
			return "\n", nil
		case "create state dir":
			return "", nil
		case "reload systemd (root)", "enable mount unit (root)":
			return "", nil
		case "check mount unit (root)":
			return "active\n", nil
		case "check path":
			return "", errInitTest("No such file")
		case "create subvolume (root)":
			return "", nil
		case "resolve WSL user":
			return "user\n", nil
		case "resolve WSL group":
			return "group\n", nil
		case "chown btrfs (root)":
			return "", nil
		default:
			return "", nil
		}
	})
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "running\n", nil
	})
	withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
		return "", nil
	})
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "Stopped\n", nil
		case "check docker pipe":
			return "False\n", nil
		case "check docker cli":
			return "", errors.New("not running")
		case "create VHDX":
			return "", nil
		default:
			return "", nil
		}
	})
	withIsElevatedStub(t, func(bool) (bool, error) { return true, nil })

	temp := t.TempDir()
	result, err := initWSL(wslInitOptions{
		Enable:      true,
		StoreSizeGB: 10,
		StorePath:   filepath.Join(temp, "btrfs.vhdx"),
	})
	if err != nil {
		t.Fatalf("initWSL: %v", err)
	}
	if !result.UseWSL || result.Distro != "Ubuntu" || result.MountDevice == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Warning == "" {
		t.Fatalf("expected warnings to be set")
	}
}
