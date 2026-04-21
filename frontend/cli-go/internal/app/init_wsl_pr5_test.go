package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/paths"
	"github.com/sqlrs/cli/internal/wsl"
)

func TestWSLBootstrapPhaseHandlesUnavailableAndRequiredModes(t *testing.T) {
	deps := defaultWSLInitDeps()
	deps.lookPath = func(string) (string, error) {
		return "", errors.New("missing")
	}

	_, err := bootstrapWSLInit(context.Background(), deps, wslInitOptions{Enable: true})
	if err == nil || !strings.Contains(err.Error(), "WSL is not available") {
		t.Fatalf("expected unavailable error, got %v", err)
	}

	result, warnErr := wslUnavailable(wslInitOptions{Require: false}, err.Error())
	if warnErr != nil {
		t.Fatalf("expected warning result, got %v", warnErr)
	}
	if result.UseWSL || !strings.Contains(result.Warning, "WSL is not available") {
		t.Fatalf("unexpected warning result: %+v", result)
	}

	_, hardErr := wslUnavailable(wslInitOptions{Require: true}, err.Error())
	if hardErr == nil || !strings.Contains(hardErr.Error(), "WSL is not available") {
		t.Fatalf("expected hard failure, got %v", hardErr)
	}
}

func TestWSLBootstrapPhaseAccumulatesDockerWarningsWithoutFailingInit(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	prev := listWSLDistrosFn
	listWSLDistrosFn = func() ([]wsl.Distro, error) {
		return []wsl.Distro{{Name: "Ubuntu", Default: true}}, nil
	}
	t.Cleanup(func() { listWSLDistrosFn = prev })

	withRunWSLCommandStub(t, func(_ context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "starting WSL distro":
			return "", nil
		case "check btrfs kernel":
			return "nodev btrfs\n", nil
		case "check btrfs-progs":
			return "/usr/sbin/mkfs.btrfs\n", nil
		case "check nsenter":
			return "/usr/bin/nsenter\n", nil
		case "check docker in WSL":
			return "", errInitTest("cannot connect to the docker daemon")
		default:
			return "", nil
		}
	})
	withRunWSLCommandAllowFailureStub(t, func(_ context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "running\n", nil
	})
	withRunHostCommandStub(t, func(_ context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "check docker desktop":
			return "Running\n", nil
		case "check docker pipe":
			return "True\n", nil
		default:
			return "", nil
		}
	})

	bootstrap, err := bootstrapWSLInit(context.Background(), defaultWSLInitDeps(), wslInitOptions{
		Enable:  true,
		Verbose: false,
	})
	if err != nil {
		t.Fatalf("bootstrapWSLInit: %v", err)
	}
	if bootstrap.Distro != "Ubuntu" {
		t.Fatalf("unexpected distro: %+v", bootstrap)
	}
	if len(bootstrap.Warnings) != 1 || !strings.Contains(bootstrap.Warnings[0], "docker is not available in WSL distro Ubuntu") {
		t.Fatalf("unexpected warnings: %+v", bootstrap.Warnings)
	}
}

func TestWSLStoragePhasePreservesReinitAndAttachSequence(t *testing.T) {
	withWindowsMode(t)
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())

	var sequence []string
	lsblkCalls := 0

	withRunWSLCommandStub(t, func(_ context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		sequence = append(sequence, "wsl:"+desc)
		switch desc {
		case "resolve XDG_STATE_HOME":
			return "/state\n", nil
		case "resolve mount unit (root)":
			return "sqlrs-state-store.mount\n", nil
		case "stop mount unit (root)", "disable mount unit (root)", "remove mount unit (root)", "reload systemd (root)":
			return "", nil
		case "findmnt (root)":
			return "", errInitTest("exit status 1")
		case "lsblk":
			lsblkCalls++
			if lsblkCalls == 1 {
				return "NAME SIZE TYPE PKNAME\nsda 1 disk\n", nil
			}
			return "NAME SIZE TYPE PKNAME\nsda 10737418240 disk\n└─sda1 10737318240 part sda\n", nil
		case "detect filesystem":
			return "btrfs\n", nil
		default:
			return "", nil
		}
	})
	withRunHostCommandStub(t, func(_ context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		sequence = append(sequence, "host:"+desc)
		return "", nil
	})
	withIsElevatedStub(t, func(bool) (bool, error) { return true, nil })

	storePath := filepath.Join(t.TempDir(), "btrfs.vhdx")
	if err := os.WriteFile(storePath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("write store path: %v", err)
	}

	storage, err := prepareWSLStorage(context.Background(), defaultWSLInitDeps(), wslInitOptions{
		Enable:      true,
		StoreSizeGB: 10,
		StorePath:   storePath,
		Reinit:      true,
	}, "Ubuntu")
	if err != nil {
		t.Fatalf("prepareWSLStorage: %v", err)
	}
	if storage.Partition != "/dev/sda1" || storage.MountUnit != "sqlrs-state-store.mount" || storage.StateDir != "/state/sqlrs/store" {
		t.Fatalf("unexpected storage result: %+v", storage)
	}

	got := strings.Join(sequence, "\n")
	for _, want := range []string{
		"wsl:stop mount unit (root)",
		"host:unmount VHDX from WSL",
		"host:create VHDX",
		"host:partition VHDX",
		"host:attach VHDX to WSL",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in sequence:\n%s", want, got)
		}
	}
	if strings.Index(got, "host:unmount VHDX from WSL") > strings.Index(got, "host:attach VHDX to WSL") {
		t.Fatalf("expected reinit before attach:\n%s", got)
	}
}

func TestWSLMountFinalizationPreservesUUIDFallbackWarnings(t *testing.T) {
	withRunWSLCommandStub(t, func(_ context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		switch desc {
		case "resolve partition UUID (root)":
			return "uuid-123\n", nil
		case "check path":
			return "", errInitTest("No such file or directory")
		case "create state dir", "reload systemd (root)", "enable mount unit (root)", "check mount unit (root)", "create subvolume (root)", "resolve WSL user", "resolve WSL group", "chown btrfs (root)":
			if desc == "check mount unit (root)" {
				return "active\n", nil
			}
			if desc == "resolve WSL user" {
				return "user\n", nil
			}
			if desc == "resolve WSL group" {
				return "group\n", nil
			}
			return "", nil
		case "findmnt (root)":
			return "btrfs\n", nil
		default:
			return "", nil
		}
	})
	withRunWSLCommandWithInputStub(t, func(_ context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
		if !strings.Contains(input, "What=/dev/sda1") {
			t.Fatalf("expected fallback mount source in unit, got %q", input)
		}
		return "", nil
	})

	mount, err := finalizeWSLMount(context.Background(), defaultWSLInitDeps(), wslInitOptions{Enable: true}, wslStoragePhase{
		Distro:    "Ubuntu",
		StateDir:  "/state/sqlrs/store",
		StorePath: "C:\\sqlrs\\store\\btrfs.vhdx",
		MountUnit: "sqlrs-state-store.mount",
		Partition: "/dev/sda1",
	})
	if err != nil {
		t.Fatalf("finalizeWSLMount: %v", err)
	}
	if mount.DeviceUUID != "uuid-123" {
		t.Fatalf("unexpected UUID: %+v", mount)
	}
	if len(mount.Warnings) != 1 || !strings.Contains(mount.Warnings[0], "/dev/disk/by-uuid/uuid-123 not found, using /dev/sda1") {
		t.Fatalf("unexpected warnings: %+v", mount.Warnings)
	}
}

func TestResolveWSLSettingsUsesSharedWSLPathHelpers(t *testing.T) {
	cfg := config.Config{}
	cfg.Engine.WSL.Mode = "required"
	cfg.Engine.WSL.Distro = "Ubuntu"
	cfg.Engine.WSL.StateDir = "/var/lib/sqlrs/store"
	cfg.Engine.WSL.Mount.Unit = "sqlrs-state-store.mount"
	cfg.Engine.WSL.EnginePath = "C:\\sqlrs\\bin\\sqlrs-engine.exe"

	daemon, runDir, statePath, stateDir, distro, mountUnit, mountFS, err := resolveWSLSettings(cfg, paths.Dirs{
		StateDir: "C:\\sqlrs\\state",
	}, "C:\\fallback\\daemon.exe")
	if err != nil {
		t.Fatalf("resolveWSLSettings: %v", err)
	}
	if daemon != "/mnt/c/sqlrs/bin/sqlrs-engine.exe" {
		t.Fatalf("unexpected daemon path: %q", daemon)
	}
	if runDir != "/var/lib/sqlrs/store/run" {
		t.Fatalf("unexpected run dir: %q", runDir)
	}
	if statePath != "/mnt/c/sqlrs/state/engine.json" {
		t.Fatalf("unexpected state path: %q", statePath)
	}
	if stateDir != "/var/lib/sqlrs/store" || distro != "Ubuntu" || mountUnit != "sqlrs-state-store.mount" || mountFS != "btrfs" {
		t.Fatalf("unexpected WSL settings: daemon=%q runDir=%q statePath=%q stateDir=%q distro=%q mountUnit=%q mountFS=%q", daemon, runDir, statePath, stateDir, distro, mountUnit, mountFS)
	}
}

func TestCleanupSpinnerRetainsVerboseAndTerminalGatingAfterSplit(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	})

	withIsTerminalWriterStub(t, func(*os.File) bool { return false })

	stop := startCleanupSpinner("inst-1", false)
	stop()
	stop = startCleanupSpinner("inst-2", true)
	stop()

	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	out := string(data)
	if !strings.Contains(out, "Deleting instance inst-1") || !strings.Contains(out, "Deleting instance inst-2") {
		t.Fatalf("expected cleanup labels in stdout, got %q", out)
	}
}

func TestInitWSLStillReturnsStableConfigFacingResult(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())

	withInitWSLBaseStubs(t, nil, nil, true, nil)

	result, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("initWSL: %v", err)
	}
	if !result.UseWSL {
		t.Fatalf("expected UseWSL true")
	}
	if result.Distro != "Ubuntu" {
		t.Fatalf("unexpected distro: %+v", result)
	}
	if result.StateDir != "/state/sqlrs/store" {
		t.Fatalf("unexpected state dir: %+v", result)
	}
	if !strings.HasSuffix(result.StorePath, "btrfs.vhdx") {
		t.Fatalf("unexpected store path: %+v", result)
	}
	if result.MountDevice != "/dev/sda1" || result.MountFSType != "btrfs" || result.MountUnit != "sqlrs-state-store.mount" || result.MountDeviceUUID != "uuid-123" {
		t.Fatalf("unexpected mount-facing result: %+v", result)
	}
	if !strings.Contains(result.Warning, "WSL restart required: wsl.exe --shutdown") {
		t.Fatalf("expected restart warning, got %+v", result)
	}
}
