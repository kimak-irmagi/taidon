package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/cli/internal/paths"
)

func TestParseDuration(t *testing.T) {
	fallback := 5 * time.Second
	out, err := ParseDuration("", fallback)
	if err != nil {
		t.Fatalf("ParseDuration: %v", err)
	}
	if out != fallback {
		t.Fatalf("expected fallback, got %v", out)
	}
	if _, err := ParseDuration("bad", fallback); err == nil {
		t.Fatalf("expected error for invalid duration")
	}
}

func TestLookupDBMSImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: postgres:15\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	image, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if !ok || image != "postgres:15" {
		t.Fatalf("unexpected image: %q (ok=%v)", image, ok)
	}
}

func TestLookupDBMSImageMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dbms:\n  image: \"\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, ok, err := LookupDBMSImage(path)
	if err != nil {
		t.Fatalf("LookupDBMSImage: %v", err)
	}
	if ok {
		t.Fatalf("expected no image")
	}
}

func TestNormalizeMap(t *testing.T) {
	input := map[any]any{
		"key": map[any]any{
			"nested": []any{"a", map[any]any{"b": "c"}},
		},
		10: "ignore",
	}
	out := normalizeMap(input).(map[string]any)
	if _, ok := out["key"]; !ok {
		t.Fatalf("expected key to be preserved")
	}
	if _, ok := out["10"]; ok {
		t.Fatalf("expected non-string key to be dropped")
	}
	nested := out["key"].(map[string]any)
	list := nested["nested"].([]any)
	if list[0].(string) != "a" {
		t.Fatalf("unexpected list: %+v", list)
	}
	inner := list[1].(map[string]any)
	if inner["b"].(string) != "c" {
		t.Fatalf("unexpected inner map: %+v", inner)
	}
}

func TestLoadConfigMergesAndResolvesPaths(t *testing.T) {
	root := t.TempDir()
	dirs := paths.Dirs{
		ConfigDir: filepath.Join(root, "config"),
		StateDir:  filepath.Join(root, "state"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	if err := os.MkdirAll(dirs.ConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	globalConfig := []byte("orchestrator:\n  daemonPath: bin/sqlrs-engine\n")
	if err := os.WriteFile(filepath.Join(dirs.ConfigDir, "config.yaml"), globalConfig, 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectDir := filepath.Join(root, "project")
	projectConfigDir := filepath.Join(projectDir, ".sqlrs")
	if err := os.MkdirAll(projectConfigDir, 0o700); err != nil {
		t.Fatalf("mkdir project config: %v", err)
	}
	projectConfig := []byte("orchestrator:\n  runDir: ./run/..\nprofiles: null\n")
	if err := os.WriteFile(filepath.Join(projectConfigDir, "config.yaml"), projectConfig, 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	loaded, err := Load(LoadOptions{WorkingDir: projectDir, Dirs: &dirs})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ProjectConfigPath == "" {
		t.Fatalf("expected project config path")
	}
	if loaded.Config.Orchestrator.RunDir == "" {
		t.Fatalf("expected runDir to be set")
	}
	if !filepath.IsAbs(loaded.Config.Orchestrator.DaemonPath) {
		t.Fatalf("expected daemonPath to be absolute, got %q", loaded.Config.Orchestrator.DaemonPath)
	}
	if loaded.Config.Profiles == nil {
		t.Fatalf("expected profiles map to be initialized")
	}
}
