package app

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseInitFlagsAdditionalValidationCoverage(t *testing.T) {
	t.Run("reject positional args", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "extra"}, ""); err == nil {
			t.Fatalf("expected positional args error")
		}
	})

	t.Run("reject invalid store size", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "--store-size", "badGB"}, ""); err == nil {
			t.Fatalf("expected store-size parse error")
		}
	})

	t.Run("reject remote url token in local mode", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "--url", "https://e", "--token", "t"}, ""); err == nil {
			t.Fatalf("expected local/remote flags mismatch error")
		}
	})

	t.Run("reject unknown snapshot", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "--snapshot", "unknown"}, ""); err == nil {
			t.Fatalf("expected unknown snapshot error")
		}
	})

	t.Run("reject unknown store type", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "--store-type", "unknown"}, ""); err == nil {
			t.Fatalf("expected unknown store type error")
		}
	})

	t.Run("reject reinit with overlay", func(t *testing.T) {
		if _, _, err := parseInitFlags([]string{"local", "--snapshot", "overlay", "--reinit"}, ""); err == nil {
			t.Fatalf("expected reinit overlay error")
		}
	})

	if runtime.GOOS == "windows" {
		t.Run("reject btrfs with dir store on windows", func(t *testing.T) {
			if _, _, err := parseInitFlags([]string{"local", "--snapshot", "btrfs", "--store-type", "dir"}, ""); err == nil {
				t.Fatalf("expected windows btrfs dir error")
			}
		})
	}
}

func TestRunInitUpdateDryRunWithValidConfigCoverage(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(marker, "config.yaml"), []byte("client:\n  output: human\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"local", "--update", "--dry-run"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !strings.Contains(out.String(), "Workspace already initialized") || !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunInitWSLAutoModeBranchCoverage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path")
	}

	prevWSL := initWSLFn
	initWSLFn = func(opts wslInitOptions) (wslInitResult, error) {
		return wslInitResult{
			UseWSL:      true,
			Distro:      "Ubuntu",
			StateDir:    "/mnt/sqlrs/store",
			EnginePath:  "/mnt/c/sqlrs/sqlrs-engine.exe",
			StorePath:   "C:\\sqlrs\\store\\btrfs.vhdx",
			MountUnit:   "sqlrs.mount",
			MountFSType: "btrfs",
		}, nil
	}
	t.Cleanup(func() { initWSLFn = prevWSL })

	workspace := t.TempDir()
	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"local"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	raw := loadConfigMap(t, filepath.Join(workspace, ".sqlrs", "config.yaml"))
	if got := nestedString(raw, "engine", "wsl", "mode"); got != "auto" {
		t.Fatalf("expected engine.wsl.mode=auto, got %q", got)
	}
}
