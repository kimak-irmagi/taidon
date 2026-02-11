package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/wsl"
)

func TestInitWSLDisabled(t *testing.T) {
	result, err := initWSL(wslInitOptions{Enable: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UseWSL {
		t.Fatalf("expected UseWSL false")
	}
}

func TestInitWSLNonWindows(t *testing.T) {
	prev := isWindows
	isWindows = false
	t.Cleanup(func() { isWindows = prev })

	_, err := initWSL(wslInitOptions{Enable: true})
	if err == nil || !strings.Contains(err.Error(), "only supported") {
		t.Fatalf("expected non-windows error, got %v", err)
	}
}

func TestInitWSLUnavailable(t *testing.T) {
	prev := os.Getenv("PATH")
	t.Setenv("PATH", "")
	t.Cleanup(func() { t.Setenv("PATH", prev) })

	result, err := initWSL(wslInitOptions{Enable: true})
	if err != nil {
		t.Fatalf("expected warning result, got error %v", err)
	}
	if result.UseWSL || result.Warning == "" {
		t.Fatalf("expected warning result, got %+v", result)
	}
}

func TestInitWSLUnavailableRequire(t *testing.T) {
	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	_, err := initWSL(wslInitOptions{Enable: true, Require: true})
	if err == nil || !strings.Contains(err.Error(), "WSL unavailable") {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestInitWSLPartitionMissing(t *testing.T) {
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
			return "NAME SIZE TYPE PKNAME\nsda 10737418240 disk\n", nil
		case "check systemd (root)":
			return "running\n", nil
		default:
			return "", nil
		}
	})
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "running\n", nil
	})
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", nil
	})
	withIsElevatedStub(t, func(bool) (bool, error) { return true, nil })

	result, err := initWSL(wslInitOptions{Enable: true, StoreSizeGB: 10})
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if result.Warning == "" {
		t.Fatalf("expected warning about partition")
	}
}

func TestInitWSLSuccess(t *testing.T) {
	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true}}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	var lsblkCalls int
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
			lsblkCalls++
			if lsblkCalls == 1 {
				return "NAME SIZE TYPE PKNAME\nsda 1 disk\n", nil
			}
			return "NAME SIZE TYPE PKNAME\nsda 10737418240 disk\n└─sda1 10737318240 part sda\n", nil
		case "detect filesystem":
			return "\n", nil
		case "wipefs (root)":
			return "", errInitTest("command not found")
		case "format btrfs (root)":
			return "", nil
		case "verify filesystem (root)":
			return "btrfs\n", nil
		case "resolve partition UUID (root)":
			return "uuid-123\n", nil
		case "check path":
			return "", errInitTest("No such file or directory")
		case "create state dir":
			return "", nil
		case "reload systemd (root)", "enable mount unit (root)", "systemctl daemon-reload":
			return "", nil
		case "check mount unit (root)":
			return "active\n", nil
		case "findmnt (root)":
			return "btrfs\n", nil
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
		switch desc {
		case "check docker desktop":
			return "", nil
		case "check docker pipe":
			return "False\n", nil
		case "check docker cli":
			return "", errInitTest("not running")
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
}

func TestListWSLDistrosError(t *testing.T) {
	prev := os.Getenv("PATH")
	t.Setenv("PATH", "")
	t.Cleanup(func() { t.Setenv("PATH", prev) })

	if _, err := listWSLDistros(); err == nil {
		t.Fatalf("expected list error")
	}
}

func TestEnsureBtrfsKernelMissingSupport(t *testing.T) {
	calls := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check btrfs kernel":
			calls++
			return "nodev ext4\n", nil
		case "load btrfs module (root)":
			return "", nil
		default:
			return "", nil
		}
	})
	if err := ensureBtrfsKernel("Ubuntu", false); err == nil {
		t.Fatalf("expected missing btrfs support error")
	}
	if calls < 2 {
		t.Fatalf("expected repeated kernel checks")
	}
}

func TestEnsureBtrfsProgsInstallError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check btrfs-progs":
			return "", errInitTest("missing")
		case "apt-get update (root)":
			return "", errInitTest("boom")
		default:
			return "", nil
		}
	})
	if err := ensureBtrfsProgs("Ubuntu", false); err == nil {
		t.Fatalf("expected install error")
	}
}

func TestEnsureNsenterInstallError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check nsenter":
			return "", errInitTest("missing")
		case "apt-get update (root)":
			return "", nil
		case "apt-get install (root)":
			return "", errInitTest("boom")
		default:
			return "", nil
		}
	})
	if err := ensureNsenter("Ubuntu", false); err == nil {
		t.Fatalf("expected install error")
	}
}

func TestResolveWSLStateStoreHomeMissing(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "resolve XDG_STATE_HOME":
			return "", errInitTest("missing")
		case "resolve HOME":
			return "", nil
		default:
			return "", nil
		}
	})
	if _, err := resolveWSLStateStore("Ubuntu", false); err == nil {
		t.Fatalf("expected HOME empty error")
	}
}

func TestResolveSystemdMountUnitEmpty(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "\n", nil
	})
	if _, err := resolveSystemdMountUnit(context.Background(), "Ubuntu", "/mnt/store", false); err == nil {
		t.Fatalf("expected empty unit error")
	}
}

func TestInstallSystemdMountUnitValidation(t *testing.T) {
	if err := installSystemdMountUnit(context.Background(), "Ubuntu", "", "/mnt/store", "/dev/sda1", "btrfs", false); err == nil {
		t.Fatalf("expected empty unit error")
	}
	if err := installSystemdMountUnit(context.Background(), "Ubuntu", "unit.mount", "", "/dev/sda1", "btrfs", false); err == nil {
		t.Fatalf("expected empty state dir error")
	}
	if err := installSystemdMountUnit(context.Background(), "Ubuntu", "unit.mount", "/mnt/store", "", "btrfs", false); err == nil {
		t.Fatalf("expected empty mount source error")
	}
}

func TestWaitForPartitionFSTypeProbeMount(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "verify filesystem (root)" {
			return "", errInitTest("exit status 1")
		}
		if desc == "probe mount dir (root)" {
			return "/tmp/sqlrs-mount-1\n", nil
		}
		if desc == "probe mount (root)" || desc == "probe umount (root)" || desc == "cleanup probe dir (root)" {
			return "", nil
		}
		return "", nil
	})
	if err := waitForPartitionFSType(context.Background(), "Ubuntu", "/dev/sda1", "btrfs", false); err != nil {
		t.Fatalf("expected probe mount success, got %v", err)
	}
}

func TestProbeBtrfsMountEmptyDir(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "probe mount dir (root)" {
			return "\n", nil
		}
		return "", nil
	})
	if err := probeBtrfsMount(context.Background(), "Ubuntu", "/dev/sda1", false); err == nil {
		t.Fatalf("expected empty mount dir error")
	}
}

func TestWslPathExistsUnknownError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "check path" {
			return "", errInitTest("permission denied")
		}
		return "", nil
	})
	_, err := wslPathExists(context.Background(), "Ubuntu", "/mnt/store", false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWslFindmntRunFallback(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" && command == "nsenter" {
			return "", errInitTest("command not found")
		}
		return "btrfs\n", nil
	})
	out, err := wslFindmntRun(context.Background(), "Ubuntu", false, []string{"-n", "-o", "FSTYPE", "-T", "/mnt/store"})
	if err != nil || strings.TrimSpace(out) != "btrfs" {
		t.Fatalf("expected fallback output, got %q err=%v", out, err)
	}
}

func TestRunWSLCommandFormatting(t *testing.T) {
	_, err := runWSLCommand(nil, "missing-distro", true, "check", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWSLCommandAllowFailureFormatting(t *testing.T) {
	_, err := runWSLCommandAllowFailure(nil, "missing-distro", true, "check", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWSLCommandWithInputFormatting(t *testing.T) {
	_, err := runWSLCommandWithInput(nil, "missing-distro", true, "check", "input", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunHostCommandFormatting(t *testing.T) {
	_, err := runHostCommand(nil, false, "check", "cmd.exe", "/c", "exit", "2")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsElevatedTrue(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc != "check admin" {
			return "", errInitTest("unexpected")
		}
		return "True\n", nil
	})
	ok, err := isElevated(false)
	if err != nil || !ok {
		t.Fatalf("expected elevated, ok=%v err=%v", ok, err)
	}
}

func TestStartSpinnerTerminal(t *testing.T) {
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no console output: %v", err)
	}
	defer out.Close()

	old := os.Stderr
	os.Stderr = out
	t.Cleanup(func() { os.Stderr = old })

	stop := startSpinner("test", false)
	time.Sleep(600 * time.Millisecond)
	stop()
}

func TestIsTerminalWriterNil(t *testing.T) {
	if isTerminalWriter(nil) {
		t.Fatalf("expected false")
	}
}

func TestRunWSLCommandWithInputUsesStdin(t *testing.T) {
	_, err := runWSLCommandWithInput(context.Background(), "missing-distro", true, "check", "input", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWSLCommandUsesRootSuffix(t *testing.T) {
	_, err := runWSLCommand(context.Background(), "missing-distro", true, "check (root)", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunWSLCommandAllowFailureUsesRootSuffix(t *testing.T) {
	_, err := runWSLCommandAllowFailure(context.Background(), "missing-distro", true, "check (root)", "true")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWslUnavailableRequireFalse(t *testing.T) {
	res, err := wslUnavailable(wslInitOptions{}, "warn")
	if err != nil || res.Warning != "warn" {
		t.Fatalf("expected warning result, got %+v err=%v", res, err)
	}
}

func TestWslUnavailableRequireTrue(t *testing.T) {
	_, err := wslUnavailable(wslInitOptions{Require: true}, "warn")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestEscapePowerShellString(t *testing.T) {
	if got := escapePowerShellString("a'b"); got != "a''b" {
		t.Fatalf("unexpected escape: %q", got)
	}
}

func TestParseLsblkNoEntries(t *testing.T) {
	if _, err := parseLsblk("\n"); err == nil {
		t.Fatalf("expected no entries error")
	}
}

func TestResolveWSLUserGroupMissing(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "resolve WSL user":
			return "user\n", nil
		case "resolve WSL group":
			return "", errInitTest("missing")
		default:
			return "", nil
		}
	})
	user, group, err := resolveWSLUser("Ubuntu", false)
	if err != nil || user != "user" || group != "" {
		t.Fatalf("unexpected user/group: %q %q err=%v", user, group, err)
	}
}

func TestReinitWSLStoreRemovesVHDX(t *testing.T) {
	temp := t.TempDir()
	vhdxPath := filepath.Join(temp, "btrfs.vhdx")
	if err := os.WriteFile(vhdxPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write vhdx: %v", err)
	}

	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "findmnt (root)":
			return "btrfs\n", nil
		case "unmount btrfs (root)":
			return "", errInitTest("exit status 32")
		default:
			return "", nil
		}
	})
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errInitTest("ignored")
	})

	if err := reinitWSLStore(context.Background(), "Ubuntu", "/mnt/store", vhdxPath, "unit.mount", false); err != nil {
		t.Fatalf("reinitWSLStore: %v", err)
	}
	if _, err := os.Stat(vhdxPath); !os.IsNotExist(err) {
		t.Fatalf("expected vhdx removed, err=%v", err)
	}
}

func TestWslMountpointUnknownError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "check mountpoint" {
			return "", errInitTest("permission denied")
		}
		return "", nil
	})
	_, err := wslMountpoint(context.Background(), "Ubuntu", "/mnt/store", false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForPartitionFSTypeErrorWithLastErr(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "verify filesystem (root)" {
			return "", errInitTest("boom")
		}
		if desc == "probe mount dir (root)" {
			return "/tmp/sqlrs-mount-1\n", nil
		}
		if desc == "probe mount (root)" {
			return "", errInitTest("boom")
		}
		if desc == "cleanup probe dir (root)" {
			return "", nil
		}
		return "", nil
	})
	if err := waitForPartitionFSType(context.Background(), "Ubuntu", "/dev/sda1", "btrfs", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWslFindmntFSTypeNotMounted(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" {
			return "", errInitTest("exit status 1")
		}
		return "", nil
	})
	fs, mounted, err := wslFindmntFSType(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil || mounted || fs != "" {
		t.Fatalf("expected not mounted, fs=%q mounted=%v err=%v", fs, mounted, err)
	}
}

func TestEnsureSystemdAvailableErrorWithState(t *testing.T) {
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "offline\n", errInitTest("boom")
	})
	if err := ensureSystemdAvailable("Ubuntu", false); err == nil {
		t.Fatalf("expected systemd error")
	}
}

func TestWaitForMountFSTypeNotMounted(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "findmnt (root)" {
			return "", errInitTest("exit status 1")
		}
		return "", nil
	})
	if err := waitForMountFSType(context.Background(), "Ubuntu", "/mnt/store", "btrfs", false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunHostCommandSuccessCoverage(t *testing.T) {
	out, err := runHostCommand(nil, true, "check", "cmd.exe", "/c", "echo", "ok")
	if err != nil || !strings.Contains(out, "ok") {
		t.Fatalf("expected output, got %q err=%v", out, err)
	}
}
