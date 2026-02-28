package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
)

func TestRunPlanKindAdditionalBranches(t *testing.T) {
	t.Run("unsupported-kind", func(t *testing.T) {
		err := runPlanKind(&bytes.Buffer{}, &bytes.Buffer{}, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), t.TempDir(), []string{"--image", "img", "--", "-c", "select 1"}, "json", "unknown")
		if err == nil || !strings.Contains(err.Error(), "unsupported plan kind") {
			t.Fatalf("expected unsupported kind error, got %v", err)
		}
	})

	t.Run("liquibase-exec-error", func(t *testing.T) {
		root := t.TempDir()
		projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(projectConfig, []byte("liquibase: ["), 0o600); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		err := runPlanKind(
			&bytes.Buffer{},
			&bytes.Buffer{},
			cli.PrepareOptions{},
			config.LoadedConfig{
				ProjectConfigPath: projectConfig,
				Paths:             paths.Dirs{ConfigDir: t.TempDir()},
			},
			root,
			root,
			[]string{"--image", "img", "--", "update"},
			"json",
			"lb",
		)
		if err == nil {
			t.Fatalf("expected liquibase exec config error")
		}
	})

	t.Run("liquibase-exec-mode-error", func(t *testing.T) {
		root := t.TempDir()
		projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(projectConfig, []byte("liquibase:\n  exec: liquibase\n"), 0o600); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		globalDir := t.TempDir()
		globalConfig := filepath.Join(globalDir, "config.yaml")
		if err := os.WriteFile(globalConfig, []byte("liquibase: ["), 0o600); err != nil {
			t.Fatalf("write global config: %v", err)
		}

		err := runPlanKind(
			&bytes.Buffer{},
			&bytes.Buffer{},
			cli.PrepareOptions{},
			config.LoadedConfig{
				ProjectConfigPath: projectConfig,
				Paths:             paths.Dirs{ConfigDir: globalDir},
			},
			root,
			root,
			[]string{"--image", "img", "--", "update"},
			"json",
			"lb",
		)
		if err == nil {
			t.Fatalf("expected liquibase exec mode error")
		}
	})

	t.Run("liquibase-normalize-args-error", func(t *testing.T) {
		root := t.TempDir()
		projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(projectConfig, []byte("liquibase:\n  exec: liquibase\n"), 0o600); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		err := runPlanKind(
			&bytes.Buffer{},
			&bytes.Buffer{},
			cli.PrepareOptions{},
			config.LoadedConfig{
				ProjectConfigPath: projectConfig,
				Paths:             paths.Dirs{ConfigDir: t.TempDir()},
			},
			root,
			root,
			[]string{"--image", "img", "--", "--search-path="},
			"json",
			"lb",
		)
		if err == nil || !strings.Contains(err.Error(), "Missing value for --search-path") {
			t.Fatalf("expected liquibase args normalization error, got %v", err)
		}
	})

	t.Run("liquibase-windows-mode-branches", func(t *testing.T) {
		root := t.TempDir()
		projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(projectConfig, []byte("liquibase:\n  exec: liquibase.bat\n"), 0o600); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		err := runPlanKind(
			&bytes.Buffer{},
			&bytes.Buffer{},
			cli.PrepareOptions{Mode: "remote"},
			config.LoadedConfig{
				ProjectConfigPath: projectConfig,
				Paths:             paths.Dirs{ConfigDir: t.TempDir()},
			},
			root,
			root,
			[]string{"--image", "img", "--", "update"},
			"json",
			"lb",
		)
		if err == nil || !strings.Contains(err.Error(), "remote mode requires explicit endpoint") {
			t.Fatalf("expected remote endpoint error after windows-mode preprocessing, got %v", err)
		}
	})

	t.Run("liquibase-normalize-workdir-error", func(t *testing.T) {
		root := t.TempDir()
		projectConfig := filepath.Join(root, ".sqlrs", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(projectConfig), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(projectConfig, []byte("liquibase:\n  exec: liquibase\n  exec_mode: native\n"), 0o600); err != nil {
			t.Fatalf("write project config: %v", err)
		}

		err := runPlanKind(
			&bytes.Buffer{},
			&bytes.Buffer{},
			cli.PrepareOptions{WSLDistro: "Ubuntu", Mode: "remote", Endpoint: "http://127.0.0.1:1"},
			config.LoadedConfig{
				ProjectConfigPath: projectConfig,
				Paths:             paths.Dirs{ConfigDir: t.TempDir()},
			},
			root,
			"relative-workdir",
			[]string{"--image", "img", "--", "update"},
			"json",
			"lb",
		)
		if err == nil || !strings.Contains(err.Error(), "path is not absolute") {
			t.Fatalf("expected normalize workdir error, got %v", err)
		}
	})
}

func TestIsWithinRelError(t *testing.T) {
	if isWithin("bad\x00base", "bad\x00target") {
		t.Fatalf("expected false when filepath.Rel fails")
	}
}
