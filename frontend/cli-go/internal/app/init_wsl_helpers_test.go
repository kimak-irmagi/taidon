package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLsblk(t *testing.T) {
	output := strings.Join([]string{
		"NAME SIZE TYPE PKNAME",
		"sda 1073741824 disk",
		"├─sda1 1072693248 part sda",
		"└─sda2 1048576 part sda",
	}, "\n")
	entries, err := parseLsblk(output)
	if err != nil {
		t.Fatalf("parseLsblk: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[1].Name != "sda1" || entries[1].PKName != "sda" || entries[1].Type != "part" {
		t.Fatalf("unexpected entry: %+v", entries[1])
	}
}

func TestParseLsblkErrors(t *testing.T) {
	if _, err := parseLsblk("sda not-a-number disk"); err == nil {
		t.Fatalf("expected invalid size error")
	}
	if _, err := parseLsblk(""); err == nil {
		t.Fatalf("expected no entries error")
	}
}

func TestCleanLsblkName(t *testing.T) {
	if got := cleanLsblkName("├─sda1"); got != "sda1" {
		t.Fatalf("unexpected clean name: %s", got)
	}
	if got := cleanLsblkName(""); got != "" {
		t.Fatalf("expected empty name")
	}
}

func TestSelectDiskBySize(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 1024 * 1024 * 1024, Type: "disk"},
		{Name: "sdb", Size: 2 * 1024 * 1024 * 1024, Type: "disk"},
	}
	if disk, err := selectDiskBySize(entries, 1024*1024*1024); err != nil || disk != "sda" {
		t.Fatalf("selectDiskBySize: disk=%s err=%v", disk, err)
	}
	if disk, err := selectDiskBySize(entries, 0); err != nil || disk != "" {
		t.Fatalf("expected no match for size 0, got %s err=%v", disk, err)
	}
	if _, err := selectDiskBySize(append(entries, lsblkEntry{Name: "sdc", Size: 1024 * 1024 * 1024, Type: "disk"}), 1024*1024*1024); err == nil {
		t.Fatalf("expected multiple disk error")
	}
}

func TestSelectPartition(t *testing.T) {
	entries := []lsblkEntry{
		{Name: "sda", Size: 1024, Type: "disk"},
		{Name: "sda1", Size: 100, Type: "part", PKName: "sda"},
		{Name: "sda2", Size: 200, Type: "part", PKName: "sda"},
	}
	part, err := selectPartition(entries, "sda")
	if err != nil || part != "sda2" {
		t.Fatalf("selectPartition: part=%s err=%v", part, err)
	}
	if _, err := selectPartition(entries, "missing"); err == nil {
		t.Fatalf("expected missing partition error")
	}
}

func TestNormalizeFSType(t *testing.T) {
	if got := normalizeFSType("btrfs\n"); got != "btrfs" {
		t.Fatalf("unexpected fstype: %s", got)
	}
	if got := normalizeFSType(" "); got != "" {
		t.Fatalf("expected empty fstype")
	}
}

func TestSanitizeWSLOutput(t *testing.T) {
	input := []byte("  value\x00\x00 ")
	if got := sanitizeWSLOutput(input); got != "value" {
		t.Fatalf("unexpected sanitized value: %s", got)
	}
}

func TestIsVHDXInUseError(t *testing.T) {
	if !isVHDXInUseError(errors.New("ObjectInUse")) {
		t.Fatalf("expected ObjectInUse to be detected")
	}
	if isVHDXInUseError(nil) {
		t.Fatalf("expected nil to be false")
	}
}

func TestIsWSLNotMountedError(t *testing.T) {
	if !isWSLNotMountedError(errors.New("not mounted")) {
		t.Fatalf("expected not mounted error to be detected")
	}
	if isWSLNotMountedError(nil) {
		t.Fatalf("expected nil to be false")
	}
}

func TestResolveHostStorePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", root)
	dir, path, err := resolveHostStorePath()
	if err != nil {
		t.Fatalf("resolveHostStorePath: %v", err)
	}
	if dir != root || filepath.Base(path) != defaultVHDXName {
		t.Fatalf("unexpected path: dir=%s path=%s", dir, path)
	}
}

func TestResolveHostStorePathLocalAppData(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", "")
	t.Setenv("LOCALAPPDATA", root)
	dir, path, err := resolveHostStorePath()
	if err != nil {
		t.Fatalf("resolveHostStorePath: %v", err)
	}
	if !strings.HasPrefix(dir, root) || filepath.Base(path) != defaultVHDXName {
		t.Fatalf("unexpected path: dir=%s path=%s", dir, path)
	}
}

func TestEnsureBtrfsProgsInstall(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check btrfs-progs":
			return "", errors.New("missing")
		case "apt-get update (root)", "apt-get install (root)":
			return "", nil
		default:
			return "", nil
		}
	})
	if err := ensureBtrfsProgs("Ubuntu", false); err != nil {
		t.Fatalf("ensureBtrfsProgs: %v", err)
	}
}

func TestEnsureSystemdAvailable(t *testing.T) {
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "running\n", nil
	})
	if err := ensureSystemdAvailable("Ubuntu", false); err != nil {
		t.Fatalf("expected running systemd, got %v", err)
	}

	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", nil
	})
	if err := ensureSystemdAvailable("Ubuntu", false); err == nil || !strings.Contains(err.Error(), "state=unknown") {
		t.Fatalf("expected unknown state error, got %v", err)
	}
}

func TestCheckDockerDesktopRunning(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "True\n", nil
		default:
			return "", errors.New("unexpected")
		}
	})
	ok, warn, err := checkDockerDesktopRunning(false)
	if err != nil || !ok || warn != "" {
		t.Fatalf("expected docker ready, got ok=%v warn=%q err=%v", ok, warn, err)
	}
}

func TestEnsureBtrfsOnPartitionMounted(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" {
			return "btrfs\n", nil
		}
		return "", nil
	})
	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition: %v", err)
	}
}

func TestEnsureBtrfsOnPartitionWrongFS(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" {
			return "ext4\n", nil
		}
		return "", nil
	})
	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err == nil {
		t.Fatalf("expected filesystem error")
	}
}

func TestEnsureBtrfsOnPartitionFormat(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			return "", errors.New("exit status 1")
		case "detect filesystem":
			return "", nil
		case "wipefs (root)":
			return "", errors.New("command not found")
		case "format btrfs (root)":
			return "", nil
		case "verify filesystem (root)":
			return "btrfs\n", nil
		default:
			return "", nil
		}
	})
	if err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false); err != nil {
		t.Fatalf("ensureBtrfsOnPartition format: %v", err)
	}
}

func TestEnsureSystemdMountUnitActive(t *testing.T) {
	step := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check mount unit (root)":
			step++
			if step == 1 {
				return "inactive\n", nil
			}
			return "active\n", nil
		case "start mount unit (root)":
			return "", nil
		case "findmnt (root)":
			return "btrfs\n", nil
		default:
			return "", nil
		}
	})
	if err := ensureSystemdMountUnitActive(context.Background(), "Ubuntu", "sqlrs.mount", "/state", "btrfs", false); err != nil {
		t.Fatalf("ensureSystemdMountUnitActive: %v", err)
	}
}

func TestResolveWSLPartitionUUID(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "uuid-1\n", nil
	})
	out, err := resolveWSLPartitionUUID(context.Background(), "Ubuntu", "/dev/sda1", false)
	if err != nil || out != "uuid-1" {
		t.Fatalf("unexpected uuid: %q err=%v", out, err)
	}
}

func TestWslMountpoint(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("not a mountpoint")
	})
	ok, err := wslMountpoint(context.Background(), "Ubuntu", "/state", false)
	if err != nil || ok {
		t.Fatalf("expected not mounted, got ok=%v err=%v", ok, err)
	}

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("boom")
	})
	if _, err := wslMountpoint(context.Background(), "Ubuntu", "/state", false); err == nil {
		t.Fatalf("expected error for unknown failure")
	}
}

func TestContainsArgs(t *testing.T) {
	args := []string{"-S", "/dev/sda1", "-T", "/state"}
	if !containsArgs(args, "-S", "/dev/sda1") {
		t.Fatalf("expected to find -S /dev/sda1")
	}
	if containsArgs(args, "-x", "missing") {
		t.Fatalf("unexpected match")
	}
}

func TestCheckDockerInWSL(t *testing.T) {
	old := runWSLCommandFn
	t.Cleanup(func() { runWSLCommandFn = old })

	runWSLCommandFn = func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("command not found")
	}
	ok, warn := checkDockerInWSL("ubuntu", false)
	if ok || !strings.Contains(warn, "docker is not installed") {
		t.Fatalf("unexpected docker warn: ok=%v warn=%s", ok, warn)
	}

	runWSLCommandFn = func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("cannot connect to the docker daemon")
	}
	ok, warn = checkDockerInWSL("ubuntu", false)
	if ok || !strings.Contains(warn, "docker is not available") {
		t.Fatalf("unexpected docker warning: ok=%v warn=%s", ok, warn)
	}
}

func TestWSLFindmntFSType(t *testing.T) {
	old := runWSLCommandFn
	t.Cleanup(func() { runWSLCommandFn = old })

	runWSLCommandFn = func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if command == "nsenter" {
			return "btrfs\n", nil
		}
		return "", nil
	}
	fs, mounted, err := wslFindmntFSType(context.Background(), "ubuntu", "/mnt", false)
	if err != nil || !mounted || fs != "btrfs" {
		t.Fatalf("unexpected findmnt: fs=%s mounted=%v err=%v", fs, mounted, err)
	}

	runWSLCommandFn = func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("exit status 1")
	}
	fs, mounted, err = wslFindmntFSType(context.Background(), "ubuntu", "/mnt", false)
	if err != nil || mounted || fs != "" {
		t.Fatalf("expected empty result for exit status 1")
	}

	calls := 0
	runWSLCommandFn = func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		calls++
		if command == "nsenter" {
			return "", errors.New("command not found")
		}
		return "ext4\n", nil
	}
	fs, mounted, err = wslFindmntFSType(context.Background(), "ubuntu", "/mnt", false)
	if err != nil || !mounted || fs != "ext4" || calls < 2 {
		t.Fatalf("expected fallback findmnt: fs=%s mounted=%v err=%v calls=%d", fs, mounted, err, calls)
	}
}

func TestEnsureHostGPTPartitionInUse(t *testing.T) {
	old := runHostCommandFn
	t.Cleanup(func() { runHostCommandFn = old })

	runHostCommandFn = func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("ObjectInUse")
	}
	if err := ensureHostGPTPartition(context.Background(), "C:\\store\\btrfs.vhdx", false); err == nil || !strings.Contains(err.Error(), "VHDX is in use") {
		t.Fatalf("expected VHDX in use error, got %v", err)
	}
}

func TestEnsureSystemdMountUnitActiveStartFailureVerbose(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check mount unit (root)":
			return "inactive\n", nil
		case "start mount unit (root)":
			return "", errors.New("start failed")
		case "mount unit logs (root)":
			return "journal tail", nil
		default:
			return "", nil
		}
	})
	err := ensureSystemdMountUnitActive(context.Background(), "Ubuntu", "sqlrs.mount", "/state", "btrfs", true)
	if err == nil || !strings.Contains(err.Error(), "journal tail") {
		t.Fatalf("expected start failure with logs, got %v", err)
	}
}

func TestEnsureSystemdMountUnitActiveStartFailureNoVerbose(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check mount unit (root)":
			return "inactive\n", nil
		case "start mount unit (root)":
			return "", errors.New("start failed")
		default:
			return "", nil
		}
	})
	err := ensureSystemdMountUnitActive(context.Background(), "Ubuntu", "sqlrs.mount", "/state", "btrfs", false)
	if err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("expected start failure, got %v", err)
	}
}

func TestResolveWSLPartitionUUIDCommandNotFound(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errors.New("command not found")
	})
	if _, err := resolveWSLPartitionUUID(context.Background(), "Ubuntu", "/dev/sda1", false); err == nil {
		t.Fatalf("expected command-not-found error")
	}
}

func TestResolveWSLPartitionUUIDEmptyOnTimeout(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	t.Cleanup(cancel)
	uuid, err := resolveWSLPartitionUUID(ctx, "Ubuntu", "/dev/sda1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uuid != "" {
		t.Fatalf("expected empty uuid, got %q", uuid)
	}
}

func TestEnsureBtrfsOnPartitionDetectNonBtrfsNoFormat(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			return "", errors.New("exit status 1")
		case "detect filesystem":
			return "ext4\n", nil
		default:
			return "", nil
		}
	})
	err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false)
	if err == nil || !strings.Contains(err.Error(), "expected btrfs") {
		t.Fatalf("expected non-btrfs error, got %v", err)
	}
}

func TestEnsureBtrfsOnPartitionWipefsFailure(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			return "", errors.New("exit status 1")
		case "detect filesystem":
			return "\n", nil
		case "wipefs (root)":
			return "", errors.New("wipefs failed")
		default:
			return "", nil
		}
	})
	err := ensureBtrfsOnPartition(context.Background(), "Ubuntu", "/dev/sda1", false, false)
	if err == nil || !strings.Contains(err.Error(), "wipefs failed") {
		t.Fatalf("expected wipefs error, got %v", err)
	}
}
