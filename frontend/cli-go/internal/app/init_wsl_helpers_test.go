package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if formatCalls != 1 {
		t.Fatalf("expected format call, got %d", formatCalls)
	}
}

func TestMountBtrfsPartitionMountsWhenNeeded(t *testing.T) {
	var mountCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "create state dir":
			return "", nil
		case "check mountpoint":
			return "", errors.New("check mountpoint: exit status 1")
		case "findmnt (root)":
			if !containsArgs(args, "-T", "/mnt/store") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "btrfs\n", nil
		case "mount btrfs (root)":
			mountCalls++
			if len(args) < 4 || args[0] != "-t" || args[1] != "btrfs" {
				return "", fmt.Errorf("unexpected mount args: %v", args)
			}
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := mountBtrfsPartition(context.Background(), "Ubuntu", "/dev/sda1", "/mnt/store", false); err != nil {
		t.Fatalf("mountBtrfsPartition: %v", err)
	}
	if mountCalls != 1 {
		t.Fatalf("expected mount call, got %d", mountCalls)
	}
}

func TestMountBtrfsPartitionVerifiesMount(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "create state dir":
			return "", nil
		case "check mountpoint":
			return "", errors.New("check mountpoint: exit status 1")
		case "mount btrfs (root)":
			return "", nil
		case "findmnt (root)":
			if !containsArgs(args, "-T", "/mnt/store") {
				return "", fmt.Errorf("unexpected findmnt args: %v", args)
			}
			return "ext4\n", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := mountBtrfsPartition(context.Background(), "Ubuntu", "/dev/sda1", "/mnt/store", false); err == nil {
		t.Fatalf("expected mount verification error")
	}
}

func TestMountBtrfsPartitionSkipsWhenMounted(t *testing.T) {
	var mountCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "create state dir":
			return "", nil
		case "check mountpoint":
			return "", nil
		case "mount btrfs (root)":
			mountCalls++
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", desc)
		}
	})

	if err := mountBtrfsPartition(context.Background(), "Ubuntu", "/dev/sda1", "/mnt/store", false); err != nil {
		t.Fatalf("mountBtrfsPartition: %v", err)
	}
	if mountCalls != 0 {
		t.Fatalf("expected no mount call, got %d", mountCalls)
	}
}

func TestEnsureBtrfsSubvolumesCreatesMissing(t *testing.T) {
	var createCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check path":
			return "", errors.New("check path: exit status 1")
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
	if createCalls != 2 {
		t.Fatalf("expected 2 create calls, got %d", createCalls)
	}
}

func TestEnsureBtrfsSubvolumesSkipsExisting(t *testing.T) {
	var createCalls int
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check path":
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
			chownCalls++
			if len(args) < 3 || args[0] != "-R" || args[1] != "user:group" {
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
