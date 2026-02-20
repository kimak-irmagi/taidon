package config

import (
	"os"
	"path/filepath"
	"testing"

	"sqlrs/cli/internal/paths"
)

func TestLoadConfigMergeAndExpand(t *testing.T) {
	temp := t.TempDir()
	dirs := paths.Dirs{
		ConfigDir: filepath.Join(temp, "config"),
		StateDir:  filepath.Join(temp, "state"),
		CacheDir:  filepath.Join(temp, "cache"),
	}

	if err := os.MkdirAll(dirs.ConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	globalConfig := []byte("defaultProfile: local\nclient:\n  timeout: 10s\norchestrator:\n  runDir: \"${StateDir}/run\"\nprofiles:\n  local:\n    mode: local\n    endpoint: auto\n    autostart: false\n")
	if err := os.WriteFile(filepath.Join(dirs.ConfigDir, "config.yaml"), globalConfig, 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectDir := filepath.Join(temp, "project", "sub")
	if err := os.MkdirAll(filepath.Join(projectDir, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir project config: %v", err)
	}

	projectConfig := []byte("client:\n  timeout: 5s\nprofiles:\n  local:\n    autostart: true\n")
	if err := os.WriteFile(filepath.Join(projectDir, ".sqlrs", "config.yaml"), projectConfig, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	result, err := Load(LoadOptions{WorkingDir: projectDir, Dirs: &dirs})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if result.Config.Client.Timeout != "5s" {
		t.Fatalf("expected project override timeout, got %q", result.Config.Client.Timeout)
	}

	expectedRunDir := filepath.Join(dirs.StateDir, "run")
	if result.Config.Orchestrator.RunDir != expectedRunDir {
		t.Fatalf("expected runDir %q, got %q", expectedRunDir, result.Config.Orchestrator.RunDir)
	}

	profile := result.Config.Profiles["local"]
	if !profile.Autostart {
		t.Fatalf("expected autostart true from project override")
	}
}

func TestLoadResolvesDaemonPathRelativeToProjectConfig(t *testing.T) {
	temp := t.TempDir()
	dirs := paths.Dirs{
		ConfigDir: filepath.Join(temp, "config"),
		StateDir:  filepath.Join(temp, "state"),
		CacheDir:  filepath.Join(temp, "cache"),
	}

	if err := os.MkdirAll(dirs.ConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	projectDir := filepath.Join(temp, "project")
	projectConfigDir := filepath.Join(projectDir, ".sqlrs")
	if err := os.MkdirAll(projectConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir project config: %v", err)
	}

	projectConfig := []byte("orchestrator:\n  daemonPath: \"../bin/sqlrs-engine\"\n")
	if err := os.WriteFile(filepath.Join(projectConfigDir, "config.yaml"), projectConfig, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	result, err := Load(LoadOptions{WorkingDir: projectDir, Dirs: &dirs})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expected := filepath.Clean(filepath.Join(projectConfigDir, "..", "bin", "sqlrs-engine"))
	if result.Config.Orchestrator.DaemonPath != expected {
		t.Fatalf("expected daemonPath %q, got %q", expected, result.Config.Orchestrator.DaemonPath)
	}
}

func TestReadConfigMapNonMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("- item\n- item2\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	data, err := readConfigMap(path)
	if err != nil {
		t.Fatalf("readConfigMap: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty map, got %+v", data)
	}
}

func TestLoadUsesGetwdAndResolveWhenOptionsAreEmpty(t *testing.T) {
	projectDir := t.TempDir()
	chdirForTest(t, projectDir)

	result, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if result.Paths.ConfigDir == "" || result.Paths.StateDir == "" || result.Paths.CacheDir == "" {
		t.Fatalf("expected resolved dirs, got %+v", result.Paths)
	}
	if result.Config.DefaultProfile == "" {
		t.Fatalf("expected default profile to be set")
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}
