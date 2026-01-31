package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveHostStorePathUsesEnv(t *testing.T) {
	t.Setenv("SQLRS_STATE_STORE", `C:\custom\store`)
	t.Setenv("LOCALAPPDATA", "")

	storeDir, storePath, err := resolveHostStorePath()
	if err != nil {
		t.Fatalf("resolveHostStorePath: %v", err)
	}
	if storeDir != `C:\custom\store` {
		t.Fatalf("expected storeDir, got %q", storeDir)
	}
	if storePath != filepath.Join(`C:\custom\store`, defaultVHDXName) {
		t.Fatalf("expected storePath, got %q", storePath)
	}
}

func TestResolveHostStorePathUsesLocalAppData(t *testing.T) {
	t.Setenv("SQLRS_STATE_STORE", "")
	t.Setenv("LOCALAPPDATA", `C:\local`)

	storeDir, storePath, err := resolveHostStorePath()
	if err != nil {
		t.Fatalf("resolveHostStorePath: %v", err)
	}
	expectedDir := filepath.Join(`C:\local`, "sqlrs", "store")
	if storeDir != expectedDir {
		t.Fatalf("expected storeDir %q, got %q", expectedDir, storeDir)
	}
	if storePath != filepath.Join(expectedDir, defaultVHDXName) {
		t.Fatalf("expected storePath, got %q", storePath)
	}
}

func TestCheckDockerDesktopRunning(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check docker desktop" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		return "Running\n", nil
	})

	ok, warn, err := checkDockerDesktopRunning(false)
	if err != nil || !ok || warn != "" {
		t.Fatalf("expected running, got ok=%v warn=%q err=%v", ok, warn, err)
	}
}

func TestCheckDockerDesktopRunningNotFound(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "False\n", nil
		case "check docker cli":
			return "", fmt.Errorf("docker not running")
		default:
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
	})

	ok, warn, err := checkDockerDesktopRunning(false)
	if err != nil || ok || warn == "" {
		t.Fatalf("expected warning, got ok=%v warn=%q err=%v", ok, warn, err)
	}
}

func TestCheckDockerDesktopRunningFallbackToCLI(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "False\n", nil
		case "check docker cli":
			return "OK", nil
		default:
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
	})

	ok, warn, err := checkDockerDesktopRunning(false)
	if err != nil || !ok || warn != "" {
		t.Fatalf("expected running, got ok=%v warn=%q err=%v", ok, warn, err)
	}
}

func TestCheckDockerDesktopRunningFallbackToPipe(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "Stopped\n", nil
		case "check docker pipe":
			return "True\n", nil
		default:
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
	})

	ok, warn, err := checkDockerDesktopRunning(false)
	if err != nil || !ok || warn != "" {
		t.Fatalf("expected running, got ok=%v warn=%q err=%v", ok, warn, err)
	}
}

func TestCheckDockerInWSLInstalled(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check docker in WSL" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		return "OK", nil
	})

	ok, warn := checkDockerInWSL("Ubuntu", false)
	if !ok || warn != "" {
		t.Fatalf("expected ok, got warn=%q", warn)
	}
}

func TestCheckDockerInWSLUnavailable(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check docker in WSL" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		return "", errInitTest("Cannot connect to the Docker daemon")
	})

	ok, warn := checkDockerInWSL("Ubuntu", false)
	if ok || warn == "" {
		t.Fatalf("expected warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestEnsureBtrfsKernelLoadsModule(t *testing.T) {
	calls := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check btrfs kernel":
			calls++
			if calls == 1 {
				return "nodev   ext4\n", nil
			}
			return "nodev   btrfs\n", nil
		case "load btrfs module (root)":
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsKernel("Ubuntu", false); err != nil {
		t.Fatalf("ensureBtrfsKernel: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected kernel check twice, got %d", calls)
	}
}

func TestParseLsblkValid(t *testing.T) {
	out := "NAME SIZE TYPE PKNAME\nsda 107374182400 disk\n├─sda1 107373182976 part sda\n"
	entries, err := parseLsblk(out)
	if err != nil {
		t.Fatalf("parseLsblk: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "sda" || entries[1].Name != "sda1" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestParseLsblkInvalidSize(t *testing.T) {
	out := "NAME SIZE TYPE PKNAME\nsda notanumber disk\n"
	if _, err := parseLsblk(out); err == nil {
		t.Fatalf("expected error for invalid size")
	}
}

func TestSelectDiskBySize(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 100 * 1024 * 1024 * 1024, Type: "disk"},
		{Name: "sda1", Size: 99 * 1024 * 1024 * 1024, Type: "part", PKName: "sda"},
	}
	disk, err := selectDiskBySize(entries, 100*1024*1024*1024)
	if err != nil {
		t.Fatalf("selectDiskBySize: %v", err)
	}
	if disk != "sda" {
		t.Fatalf("expected sda, got %q", disk)
	}
}

func TestSelectDiskBySizeMultiple(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 100 * 1024 * 1024 * 1024, Type: "disk"},
		{Name: "sdb", Size: 100 * 1024 * 1024 * 1024, Type: "disk"},
	}
	if _, err := selectDiskBySize(entries, 100*1024*1024*1024); err == nil {
		t.Fatalf("expected error for multiple matches")
	}
}

func TestSelectDiskBySizeNone(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 10 * 1024 * 1024, Type: "disk"},
	}
	disk, err := selectDiskBySize(entries, 100*1024*1024*1024)
	if err != nil {
		t.Fatalf("selectDiskBySize: %v", err)
	}
	if disk != "" {
		t.Fatalf("expected empty disk, got %q", disk)
	}
}

func TestSelectPartition(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 100, Type: "disk"},
		{Name: "sda1", Size: 1, Type: "part", PKName: "sda"},
		{Name: "sda2", Size: 99, Type: "part", PKName: "sda"},
	}
	part, err := selectPartition(entries, "sda")
	if err != nil {
		t.Fatalf("selectPartition: %v", err)
	}
	if part != "sda2" {
		t.Fatalf("expected sda2, got %q", part)
	}
}

func TestFindWSLDisk(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "lsblk" {
			return "NAME SIZE TYPE PKNAME\nsda 107374182400 disk\n└─sda1 107373182976 part sda\n", nil
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	disk, part, err := findWSLDisk(context.Background(), "Ubuntu", 100*1024*1024*1024, false)
	if err != nil {
		t.Fatalf("findWSLDisk: %v", err)
	}
	if disk != "/dev/sda" || part != "/dev/sda1" {
		t.Fatalf("unexpected disk/part: %s %s", disk, part)
	}
}

func TestEnsureBtrfsOnPartitionSkipsWhenBtrfs(t *testing.T) {
	var formatCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			if !containsArgs(args, "-S", "/dev/sda1") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "", errInitTest("exit status 1")
		case "detect filesystem":
			return "btrfs\n", nil
		case "format btrfs (root)":
			formatCalls++
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", true, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition: %v", err)
	}
	if formatCalls != 0 {
		t.Fatalf("expected no format calls, got %d", formatCalls)
	}
}

func TestEnsureBtrfsOnPartitionFormatsWhenNeeded(t *testing.T) {
	var formatCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			if !containsArgs(args, "-S", "/dev/sda1") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "", errInitTest("exit status 1")
		case "detect filesystem":
			return "ext4\n", nil
		case "wipefs (root)":
			return "", nil
		case "format btrfs (root)":
			formatCalls++
			return "", nil
		case "verify filesystem (root)":
			return "btrfs\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", true, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition: %v", err)
	}
	if formatCalls != 1 {
		t.Fatalf("expected format call, got %d", formatCalls)
	}
}

func TestEnsureBtrfsSubvolumesCreatesMissing(t *testing.T) {
	var createCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check path":
			if command != "nsenter" {
				return "", fmt.Errorf("expected nsenter, got %s", command)
			}
			return "", errors.New("check path: exit status 1")
		case "create subvolume (root)":
			if command != "nsenter" {
				return "", fmt.Errorf("expected nsenter, got %s", command)
			}
			createCalls++
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsSubvolumes(context.Background(), "Ubuntu", "/mnt/store", false); err != nil {
		t.Fatalf("ensureBtrfsSubvolumes: %v", err)
	}
	if createCalls != 2 {
		t.Fatalf("expected 2 create calls, got %d", createCalls)
	}
}

func TestEnsureBtrfsSubvolumesSkipsExisting(t *testing.T) {
	var createCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check path":
			if command != "nsenter" {
				return "", fmt.Errorf("expected nsenter, got %s", command)
			}
			return "", nil
		case "create subvolume (root)":
			createCalls++
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsSubvolumes(context.Background(), "Ubuntu", "/mnt/store", false); err != nil {
		t.Fatalf("ensureBtrfsSubvolumes: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no create calls, got %d", createCalls)
	}
}

func TestEnsureBtrfsOwnership(t *testing.T) {
	var chownCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "resolve WSL user":
			return "user\n", nil
		case "resolve WSL group":
			return "group\n", nil
		case "chown btrfs (root)":
			if command != "nsenter" {
				return "", fmt.Errorf("expected nsenter, got %s", command)
			}
			chownCalls++
			if !containsArg(args, "-R") || !containsArg(args, "user:group") {
				return "", fmt.Errorf("unexpected chown args: %v", args)
			}
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOwnership(context.Background(), "Ubuntu", "/mnt/store", false); err != nil {
		t.Fatalf("ensureBtrfsOwnership: %v", err)
	}
	if chownCalls != 1 {
		t.Fatalf("expected chown call, got %d", chownCalls)
	}
}

func TestResolveWSLUserMissing(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "resolve WSL user" {
			return "\n", nil
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	if _, _, err := resolveWSLUser("Ubuntu", false); err == nil {
		t.Fatalf("expected error for empty user")
	}
}

func TestWSLMountpointExitStatus32(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "check mountpoint" {
			return "", errInitTest("exit status 32")
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	mounted, err := wslMountpoint(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil {
		t.Fatalf("wslMountpoint: %v", err)
	}
	if mounted {
		t.Fatalf("expected not mounted")
	}
}

func TestIsWSLNotMountedError(t *testing.T) {
	if !isWSLNotMountedError(errInitTest("exit status 32")) {
		t.Fatalf("expected exit status 32 to be treated as not mounted")
	}
	if !isWSLNotMountedError(errInitTest("not mounted")) {
		t.Fatalf("expected not mounted text to be treated as not mounted")
	}
	if isWSLNotMountedError(errInitTest("other")) {
		t.Fatalf("expected other errors to be false")
	}
}

func TestEnsureHostVHDXUsesExisting(t *testing.T) {
	dir := t.TempDir()
	vhdxPath := filepath.Join(dir, "btrfs.vhdx")
	if err := os.WriteFile(vhdxPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write vhdx: %v", err)
	}

	var calls int
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		calls++
		return "", nil
	})

	created, err := ensureHostVHDX(context.Background(), vhdxPath, 100, false)
	if err != nil {
		t.Fatalf("ensureHostVHDX: %v", err)
	}
	if created {
		t.Fatalf("expected not created")
	}
	if calls != 0 {
		t.Fatalf("expected no host command calls, got %d", calls)
	}
}

func TestEnsureHostVHDXCreatesNew(t *testing.T) {
	dir := t.TempDir()
	vhdxPath := filepath.Join(dir, "btrfs.vhdx")

	var calls int
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "create VHDX" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		calls++
		return "", nil
	})

	created, err := ensureHostVHDX(context.Background(), vhdxPath, 100, false)
	if err != nil {
		t.Fatalf("ensureHostVHDX: %v", err)
	}
	if !created {
		t.Fatalf("expected created")
	}
	if calls != 1 {
		t.Fatalf("expected create call, got %d", calls)
	}
}

func TestEnsureHostGPTPartitionCallsHostCommand(t *testing.T) {
	var calls int
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "partition VHDX" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		calls++
		return "", nil
	})

	if err := ensureHostGPTPartition(context.Background(), `C:\temp\btrfs.vhdx`, false); err != nil {
		t.Fatalf("ensureHostGPTPartition: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected partition call, got %d", calls)
	}
}

func TestAttachVHDXToWSLCallsHostCommand(t *testing.T) {
	var calls int
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "attach VHDX to WSL" {
			return "", fmt.Errorf("unexpected desc: %s", desc)
		}
		calls++
		return "", nil
	})

	if err := attachVHDXToWSL(context.Background(), `C:\temp\btrfs.vhdx`, "Ubuntu", false); err != nil {
		t.Fatalf("attachVHDXToWSL: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected attach call, got %d", calls)
	}
}

func TestEnsureBtrfsOnPartitionRejectsWithoutReinit(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" {
			return "", errInitTest("exit status 1")
		}
		if desc == "detect filesystem" {
			return "ext4\n", nil
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err == nil {
		t.Fatalf("expected error when formatting not allowed")
	}
}

func TestEnsureBtrfsOnPartitionFormatsWhenNoFS(t *testing.T) {
	var formatCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			if !containsArgs(args, "-S", "/dev/sda1") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "", errInitTest("exit status 1")
		case "detect filesystem":
			return "\n", nil
		case "wipefs (root)":
			return "", nil
		case "format btrfs (root)":
			formatCalls++
			return "", nil
		case "verify filesystem (root)":
			return "btrfs\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition: %v", err)
	}
	if formatCalls != 1 {
		t.Fatalf("expected format call, got %d", formatCalls)
	}
}

func TestEnsureBtrfsOnPartitionSkipsWhenMountedBtrfs(t *testing.T) {
	var formatCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			if !containsArgs(args, "-S", "/dev/sda1") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "btrfs\n", nil
		case "format btrfs (root)":
			formatCalls++
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition: %v", err)
	}
	if formatCalls != 0 {
		t.Fatalf("expected no format call, got %d", formatCalls)
	}
}

func TestEnsureBtrfsOnPartitionMountedWrongFS(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			if !containsArgs(args, "-S", "/dev/sda1") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "ext4\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnsureBtrfsProgsSkipsWhenPresent(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "check btrfs-progs" {
			return "/usr/sbin/mkfs.btrfs\n", nil
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	if err := ensureBtrfsProgs("Ubuntu", false); err != nil {
		t.Fatalf("ensureBtrfsProgs: %v", err)
	}
}

func TestEnsureBtrfsProgsInstallsAsRoot(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check btrfs-progs":
			return "", errInitTest("missing")
		case "apt-get update (root)":
			return "", nil
		case "apt-get install (root)":
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureBtrfsProgs("Ubuntu", false); err != nil {
		t.Fatalf("ensureBtrfsProgs: %v", err)
	}
}

func TestEnsureSystemdAvailableRunning(t *testing.T) {
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check systemd (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "running\n", nil
	})

	if err := ensureSystemdAvailable("Ubuntu", false); err != nil {
		t.Fatalf("ensureSystemdAvailable: %v", err)
	}
}

func TestEnsureSystemdAvailableNotRunning(t *testing.T) {
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check systemd (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "offline\n", nil
	})

	if err := ensureSystemdAvailable("Ubuntu", false); err == nil {
		t.Fatalf("expected systemd error")
	}
}

func TestEnsureSystemdAvailableDegradedWithError(t *testing.T) {
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check systemd (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "degraded\n", errInitTest("exit status 1")
	})

	if err := ensureSystemdAvailable("Ubuntu", false); err != nil {
		t.Fatalf("expected degraded to be accepted, got %v", err)
	}
}

func TestResolveSystemdMountUnit(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "resolve mount unit (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "sqlrs-state-store.mount\n", nil
	})

	unit, err := resolveSystemdMountUnit(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil {
		t.Fatalf("resolveSystemdMountUnit: %v", err)
	}
	if unit != "sqlrs-state-store.mount" {
		t.Fatalf("unexpected unit: %s", unit)
	}
}

func TestResolveWSLPartitionUUID(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "resolve partition UUID (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "uuid-123\n", nil
	})

	uuid, err := resolveWSLPartitionUUID(context.Background(), "Ubuntu", "/dev/sda1", false)
	if err != nil {
		t.Fatalf("resolveWSLPartitionUUID: %v", err)
	}
	if uuid != "uuid-123" {
		t.Fatalf("unexpected uuid: %s", uuid)
	}
}

func TestResolveWSLPartitionUUIDEmptyReturnsEmpty(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "resolve partition UUID (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "\n", nil
	})

	uuid, err := resolveWSLPartitionUUID(ctx, "Ubuntu", "/dev/sda1", false)
	if err != nil {
		t.Fatalf("resolveWSLPartitionUUID: %v", err)
	}
	if uuid != "" {
		t.Fatalf("expected empty uuid, got %q", uuid)
	}
}

func TestResolveWSLPartitionUUIDCommandMissing(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "resolve partition UUID (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "", errInitTest("command not found")
	})

	if _, err := resolveWSLPartitionUUID(context.Background(), "Ubuntu", "/dev/sda1", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInstallSystemdMountUnitWritesUnit(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "create state dir":
			return "", nil
		case "reload systemd (root)", "enable mount unit (root)":
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})
	withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
		if desc != "write mount unit (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		if !strings.Contains(input, "Where=/mnt/store") || !strings.Contains(input, "What=/dev/disk/by-uuid/UUID-123") {
			t.Fatalf("unexpected unit content: %q", input)
		}
		return "", nil
	})

	if err := installSystemdMountUnit(context.Background(), "Ubuntu", "sqlrs-state-store.mount", "/mnt/store", "/dev/disk/by-uuid/UUID-123", "btrfs", false); err != nil {
		t.Fatalf("installSystemdMountUnit: %v", err)
	}
}

func TestEnsureSystemdMountUnitActiveStarts(t *testing.T) {
	checkCalls := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check mount unit (root)":
			checkCalls++
			if checkCalls == 1 {
				return "inactive\n", errInitTest("exit status 3")
			}
			return "active\n", nil
		case "start mount unit (root)":
			return "", nil
		case "findmnt (root)":
			return "btrfs\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureSystemdMountUnitActive(context.Background(), "Ubuntu", "sqlrs-state-store.mount", "/mnt/store", "btrfs", false); err != nil {
		t.Fatalf("ensureSystemdMountUnitActive: %v", err)
	}
}

func TestEnsureSystemdMountUnitActiveWrongFS(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check mount unit (root)":
			return "active\n", nil
		case "findmnt (root)":
			return "ext4\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := ensureSystemdMountUnitActive(context.Background(), "Ubuntu", "sqlrs-state-store.mount", "/mnt/store", "btrfs", false); err == nil {
		t.Fatalf("expected fstype error")
	}
}

func TestWaitForMountFSTypeRetriesUntilMatch(t *testing.T) {
	var calls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "findmnt (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		calls++
		if calls < 2 {
			return "ext4\n", nil
		}
		return "btrfs\n", nil
	})

	if err := waitForMountFSType(context.Background(), "Ubuntu", "/mnt/store", "btrfs", false); err != nil {
		t.Fatalf("waitForMountFSType: %v", err)
	}
}

func TestWaitForMountFSTypeReturnsError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "findmnt (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "ext4\n", nil
	})

	if err := waitForMountFSType(context.Background(), "Ubuntu", "/mnt/store", "btrfs", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForPartitionFSTypeMatches(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "verify filesystem (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "btrfs\n", nil
	})

	if err := waitForPartitionFSType(context.Background(), "Ubuntu", "/dev/sda1", "btrfs", false); err != nil {
		t.Fatalf("waitForPartitionFSType: %v", err)
	}
}

func TestWaitForPartitionFSTypeErrors(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "verify filesystem (root)" {
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
		return "ext4\n", nil
	})

	if err := waitForPartitionFSType(context.Background(), "Ubuntu", "/dev/sda1", "btrfs", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProbeBtrfsMountSuccess(t *testing.T) {
	var created bool
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "probe mount dir (root)":
			created = true
			return "/tmp/sqlrs-mount-123\n", nil
		case "probe mount (root)":
			return "", nil
		case "probe umount (root)":
			return "", nil
		case "cleanup probe dir (root)":
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := probeBtrfsMount(context.Background(), "Ubuntu", "/dev/sda1", false); err != nil {
		t.Fatalf("probeBtrfsMount: %v", err)
	}
	if !created {
		t.Fatalf("expected probe dir creation")
	}
}

func TestIsVHDXInUseError(t *testing.T) {
	if !isVHDXInUseError(errInitTest("ObjectInUse")) {
		t.Fatalf("expected ObjectInUse to match")
	}
	if !isVHDXInUseError(errInitTest("in use by another process")) {
		t.Fatalf("expected in use text to match")
	}
	if !isVHDXInUseError(errInitTest("0x80070020")) {
		t.Fatalf("expected error code to match")
	}
	if isVHDXInUseError(errInitTest("other")) {
		t.Fatalf("expected no match")
	}
}

func TestResolveWSLStateStoreUsesXDG(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "resolve XDG_STATE_HOME" {
			return "/state", nil
		}
		return "", fmt.Errorf("unexpected command: %s", desc)
	})

	path, err := resolveWSLStateStore("Ubuntu", false)
	if err != nil {
		t.Fatalf("resolveWSLStateStore: %v", err)
	}
	if path != "/state/sqlrs/store" {
		t.Fatalf("unexpected state dir: %s", path)
	}
}

func TestResolveWSLStateStoreFallsBackToHome(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "resolve XDG_STATE_HOME":
			return "", fmt.Errorf("missing")
		case "resolve HOME":
			return "/home/user", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	path, err := resolveWSLStateStore("Ubuntu", false)
	if err != nil {
		t.Fatalf("resolveWSLStateStore: %v", err)
	}
	if path != "/home/user/.local/state/sqlrs/store" {
		t.Fatalf("unexpected state dir: %s", path)
	}
}

func TestSanitizeWSLOutputRemovesNulls(t *testing.T) {
	out := sanitizeWSLOutput([]byte(" /home/user \x00\x00"))
	if out != "/home/user" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCleanLsblkName(t *testing.T) {
	value := cleanLsblkName("├─sde1")
	if value != "sde1" {
		t.Fatalf("expected sde1, got %q", value)
	}
}

func withRunWSLCommandStub(t *testing.T, fn func(context.Context, string, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandFn
	runWSLCommandFn = fn
	t.Cleanup(func() {
		runWSLCommandFn = prev
	})
}

func withRunHostCommandStub(t *testing.T, fn func(context.Context, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runHostCommandFn
	runHostCommandFn = fn
	t.Cleanup(func() {
		runHostCommandFn = prev
	})
}

func withRunWSLCommandWithInputStub(t *testing.T, fn func(context.Context, string, bool, string, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandWithInputFn
	runWSLCommandWithInputFn = fn
	t.Cleanup(func() {
		runWSLCommandWithInputFn = prev
	})
}

func withRunWSLCommandAllowFailureStub(t *testing.T, fn func(context.Context, string, bool, string, string, ...string) (string, error)) {
	t.Helper()
	prev := runWSLCommandAllowFailureFn
	runWSLCommandAllowFailureFn = fn
	t.Cleanup(func() {
		runWSLCommandAllowFailureFn = prev
	})
}

func withIsElevatedStub(t *testing.T, fn func(bool) (bool, error)) {
	t.Helper()
	prev := isElevatedFn
	isElevatedFn = fn
	t.Cleanup(func() {
		isElevatedFn = prev
	})
}

func containsArgs(args []string, flag string, value string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
		if strings.HasPrefix(args[i], flag+"=") && strings.TrimPrefix(args[i], flag+"=") == value {
			return true
		}
	}
	return false
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}
