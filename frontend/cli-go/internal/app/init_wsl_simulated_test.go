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

type initWSLStubReply struct {
	Out     string
	Err     error
	Handled bool
}

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

func withInitWSLBaseStubs(
	t *testing.T,
	wslOverride func(desc, command string, args []string) initWSLStubReply,
	hostOverride func(desc, command string, args []string) initWSLStubReply,
	elevated bool,
	elevatedErr error,
) {
	t.Helper()

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true}}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if wslOverride != nil {
			reply := wslOverride(desc, command, args)
			if reply.Handled {
				return reply.Out, reply.Err
			}
		}
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
		case "resolve partition UUID (root)":
			return "uuid-123\n", nil
		case "check path":
			return "", nil
		case "create state dir":
			return "", nil
		case "reload systemd (root)", "enable mount unit (root)":
			return "", nil
		case "check mount unit (root)":
			return "active\n", nil
		case "check path (root)":
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
		if hostOverride != nil {
			reply := hostOverride(desc, command, args)
			if reply.Handled {
				return reply.Out, reply.Err
			}
		}
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "True\n", nil
		case "check docker cli":
			return "", nil
		case "create VHDX":
			return "", nil
		case "partition VHDX":
			return "", nil
		case "attach VHDX to WSL":
			return "", nil
		default:
			return "", nil
		}
	})
	withIsElevatedStub(t, func(bool) (bool, error) {
		return elevated, elevatedErr
	})
}

func defaultInitWSLOpts(t *testing.T) wslInitOptions {
	t.Helper()
	return wslInitOptions{
		Enable:      true,
		StoreSizeGB: 10,
		StorePath:   filepath.Join(t.TempDir(), "btrfs.vhdx"),
	}
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

func TestInitWSLSelectDistroFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	res, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "distro resolution failed") {
		t.Fatalf("expected distro warning, got %+v", res)
	}
}

func TestInitWSLStartFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "starting WSL distro" {
			return initWSLStubReply{Err: errors.New("boom"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "distro start failed") {
		t.Fatalf("expected start warning, got %+v", res)
	}
}

func TestInitWSLMountUnitResolutionFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "resolve mount unit (root)" {
			return initWSLStubReply{Err: errors.New("boom"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "mount unit resolution failed") {
		t.Fatalf("expected mount unit warning, got %+v", res)
	}
}

func TestInitWSLDevicePathCheckErrorSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "check path" {
			return initWSLStubReply{Err: errors.New("permission denied"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(wslInitOptions{Enable: true, StoreSizeGB: 10, StorePath: filepath.Join(t.TempDir(), "btrfs.vhdx")})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "device path check failed") {
		t.Fatalf("expected path check warning, got %+v", res)
	}
}

func TestInitWSLDevicePathFallbackSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "check path" {
			return initWSLStubReply{Err: errInitTest("exit status 1"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(wslInitOptions{Enable: true, StoreSizeGB: 10, StorePath: filepath.Join(t.TempDir(), "btrfs.vhdx")})
	if err != nil {
		t.Fatalf("expected success result, got %v", err)
	}
	if !res.UseWSL {
		t.Fatalf("expected WSL success, got %+v", res)
	}
	if !strings.Contains(res.Warning, "not found, using /dev/sda1") {
		t.Fatalf("expected fallback warning, got %q", res.Warning)
	}
}

func TestInitWSLKernelFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "check btrfs kernel" {
			return initWSLStubReply{Out: "nodev ext4\n", Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "btrfs kernel support missing") {
		t.Fatalf("expected kernel warning, got %+v", res)
	}
}

func TestInitWSLAdminCheckErrorSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)
	withInitWSLBaseStubs(t, nil, nil, false, errors.New("admin failed"))

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "Administrator check failed") {
		t.Fatalf("expected admin warning, got %+v", res)
	}
}

func TestInitWSLStateDirResolutionFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "resolve XDG_STATE_HOME":
			return initWSLStubReply{Err: errors.New("missing"), Handled: true}
		case "resolve HOME":
			return initWSLStubReply{Out: "\n", Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "state dir resolution failed") {
		t.Fatalf("expected state dir warning, got %+v", res)
	}
}

func TestInitWSLReinitFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "findmnt (root)" {
			return initWSLStubReply{Err: errors.New("boom"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	opts := defaultInitWSLOpts(t)
	opts.Reinit = true
	res, err := initWSL(opts)
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "reinit failed") {
		t.Fatalf("expected reinit warning, got %+v", res)
	}
}

func TestInitWSLVHDXInitFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, nil, func(desc, command string, args []string) initWSLStubReply {
		if desc == "create VHDX" {
			return initWSLStubReply{Err: errors.New("create failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "VHDX init failed") {
		t.Fatalf("expected VHDX init warning, got %+v", res)
	}
}

func TestInitWSLDiskDetectionFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "lsblk" {
			return initWSLStubReply{Err: errors.New("lsblk failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "disk detection failed") {
		t.Fatalf("expected disk detection warning, got %+v", res)
	}
}

func TestInitWSLPartitioningFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "lsblk" {
			return initWSLStubReply{Out: "NAME SIZE TYPE PKNAME\nsda 1 disk\n", Handled: true}
		}
		return initWSLStubReply{}
	}, func(desc, command string, args []string) initWSLStubReply {
		if desc == "partition VHDX" {
			return initWSLStubReply{Err: errors.New("partition failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "partitioning failed") {
		t.Fatalf("expected partition warning, got %+v", res)
	}
}

func TestInitWSLAttachFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "lsblk" {
			return initWSLStubReply{Out: "NAME SIZE TYPE PKNAME\nsda 1 disk\n", Handled: true}
		}
		return initWSLStubReply{}
	}, func(desc, command string, args []string) initWSLStubReply {
		if desc == "attach VHDX to WSL" {
			return initWSLStubReply{Err: errors.New("attach failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "WSL mount failed") {
		t.Fatalf("expected attach warning, got %+v", res)
	}
}

func TestInitWSLBtrfsFormatFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "findmnt (root)" {
			return initWSLStubReply{Out: "ext4\n", Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "btrfs format failed") {
		t.Fatalf("expected btrfs warning, got %+v", res)
	}
}
