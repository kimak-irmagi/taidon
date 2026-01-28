package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitUpdateExistingWorkspaceAppliesOverrides(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	if err := os.WriteFile(configPath, []byte("client:\n  output: human\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	enginePath := filepath.Join(workspace, "bin", "sqlrs-engine")
	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update", "--engine", enginePath, "--shared-cache"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "orchestrator", "daemonPath"); got != enginePath {
		t.Fatalf("expected daemonPath %q, got %q", enginePath, got)
	}
	cache, ok := raw["cache"].(map[string]any)
	if !ok || cache["shared"] != true {
		t.Fatalf("expected cache.shared true, got %v", raw["cache"])
	}
}

func TestInitUpdateExistingWorkspaceWSL(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	if err := os.WriteFile(configPath, []byte("client:\n  output: human\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		return wslInitResult{UseWSL: true, Distro: "Ubuntu", StateDir: "/var/lib/sqlrs"}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update", "--wsl"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "engine", "wsl", "mode"); got != "auto" {
		t.Fatalf("expected mode auto, got %q", got)
	}
	if got := nestedString(raw, "engine", "wsl", "distro"); got != "Ubuntu" {
		t.Fatalf("expected distro Ubuntu, got %q", got)
	}
	if got := nestedString(raw, "engine", "wsl", "stateDir"); got != "/var/lib/sqlrs" {
		t.Fatalf("expected stateDir, got %q", got)
	}
}

func TestInitUpdateExistingWorkspaceNoFlagsNoChange(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	original := []byte("client:\n  output: json\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(updated) != string(original) {
		t.Fatalf("expected config unchanged")
	}
}

func TestInitUpdateCreatesWorkspaceWhenMissing(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !dirExists(filepath.Join(workspace, ".sqlrs")) {
		t.Fatalf("expected workspace marker to exist")
	}
}

func TestInitUpdateMissingConfigCreatesNew(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !fileExists(configPath) {
		t.Fatalf("expected config to be created")
	}
}

func TestInitUpdateCorruptConfigRecreates(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	if err := os.WriteFile(configPath, []byte("key: ["), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--update"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("expected valid yaml, got %v", err)
	}
}
