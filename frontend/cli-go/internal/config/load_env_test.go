package config

import (
	"os"
	"path/filepath"
	"testing"

	"sqlrs/cli/internal/paths"
)

func TestLoadUsesSQLRSROOT(t *testing.T) {
	temp := t.TempDir()
	dirs := paths.Dirs{
		ConfigDir: filepath.Join(temp, "config"),
		StateDir:  filepath.Join(temp, "state"),
		CacheDir:  filepath.Join(temp, "cache"),
	}

	if err := os.MkdirAll(dirs.ConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	if err := os.Setenv("SQLRSROOT", filepath.Join(temp, "root")); err != nil {
		t.Fatalf("set env: %v", err)
	}
	defer os.Unsetenv("SQLRSROOT")

	globalConfig := []byte("orchestrator:\n  runDir: \"${SQLRSROOT}/run\"\n")
	if err := os.WriteFile(filepath.Join(dirs.ConfigDir, "config.yaml"), globalConfig, 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	result, err := Load(LoadOptions{WorkingDir: temp, Dirs: &dirs})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expected := filepath.Clean(filepath.Join(temp, "root", "run"))
	if result.Config.Orchestrator.RunDir != expected {
		t.Fatalf("expected runDir %q, got %q", expected, result.Config.Orchestrator.RunDir)
	}
}
