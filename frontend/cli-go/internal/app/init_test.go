package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitCreatesWorkspace(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	if err := runInit(&out, workspace, "", nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	marker := filepath.Join(workspace, ".sqlrs")
	if !dirExists(marker) {
		t.Fatalf("expected %s to exist", marker)
	}

	configPath := filepath.Join(marker, "config.yaml")
	if !fileExists(configPath) {
		t.Fatalf("expected %s to exist", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
}

func TestInitRejectsNestedWorkspace(t *testing.T) {
	parent := t.TempDir()
	if err := os.MkdirAll(filepath.Join(parent, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("create parent marker: %v", err)
	}
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("create child: %v", err)
	}

	var out bytes.Buffer
	err := runInit(&out, child, "", nil)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}

func TestInitDryRunDoesNotCreate(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	if err := runInit(&out, workspace, "", []string{"--dry-run"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	marker := filepath.Join(workspace, ".sqlrs")
	if dirExists(marker) {
		t.Fatalf("expected %s to not exist", marker)
	}
}

func TestInitRejectsCorruptConfig(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("create marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	if err := os.WriteFile(configPath, []byte("key: ["), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	err := runInit(&out, workspace, "", nil)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 3 {
		t.Fatalf("expected exit code 3, got %d", exitErr.Code)
	}
}

func TestInitWritesOverrides(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	absEngine := filepath.Join(workspace, "bin", "sqlrs-engine")
	err := runInit(&out, workspace, "", []string{
		"--engine", absEngine,
		"--shared-cache",
	})
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}

	orchestrator, ok := raw["orchestrator"].(map[string]any)
	if !ok {
		t.Fatalf("expected orchestrator map")
	}
	if orchestrator["daemonPath"] != absEngine {
		t.Fatalf("expected daemonPath override, got %v", orchestrator["daemonPath"])
	}

	cache, ok := raw["cache"].(map[string]any)
	if !ok {
		t.Fatalf("expected cache map")
	}
	if cache["shared"] != true {
		t.Fatalf("expected cache.shared true, got %v", cache["shared"])
	}
}

func TestInitRelativeEnginePathWithinWorkspace(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	relEngine := filepath.Join("bin", "sqlrs-engine")
	err := runInit(&out, workspace, "", []string{"--engine", relEngine})
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}

	orchestrator, ok := raw["orchestrator"].(map[string]any)
	if !ok {
		t.Fatalf("expected orchestrator map")
	}

	configDir := filepath.Join(workspace, ".sqlrs")
	absEngine := filepath.Join(workspace, relEngine)
	expected, err := filepath.Rel(configDir, absEngine)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}
	if orchestrator["daemonPath"] != expected {
		t.Fatalf("expected daemonPath %q, got %v", expected, orchestrator["daemonPath"])
	}
}

func TestInitRelativeEnginePathOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	var out bytes.Buffer

	relEngine := filepath.Join("bin", "sqlrs-engine")
	err := runInit(&out, outside, "", []string{
		"--workspace", workspace,
		"--engine", relEngine,
	})
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}

	orchestrator, ok := raw["orchestrator"].(map[string]any)
	if !ok {
		t.Fatalf("expected orchestrator map")
	}

	actual, ok := orchestrator["daemonPath"].(string)
	if !ok {
		t.Fatalf("expected daemonPath string, got %T", orchestrator["daemonPath"])
	}

	outsideAbs := resolveExistingPath(outside)
	expected := filepath.Clean(filepath.Join(outsideAbs, relEngine))
	if !pathsEquivalent(expected, actual) {
		t.Fatalf("expected daemonPath %q, got %v", expected, orchestrator["daemonPath"])
	}
}

func TestParseInitFlagsHelp(t *testing.T) {
	_, showHelp, err := parseInitFlags([]string{"--help"}, "")
	if err != nil || !showHelp {
		t.Fatalf("expected help, got err=%v help=%v", err, showHelp)
	}
}

func TestParseInitFlagsInvalidArgs(t *testing.T) {
	_, _, err := parseInitFlags([]string{"--unknown"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestResolveWorkspacePathRejectsFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := resolveWorkspacePath(filePath, dir); err == nil {
		t.Fatalf("expected error for file path")
	}
}

func TestRunInitExistingWorkspaceWithoutConfig(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !strings.Contains(out.String(), "Workspace already initialized") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunInitRejectsMissingWorkspace(t *testing.T) {
	var out bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing")
	err := runInit(&out, missing, "", nil)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 4 {
		t.Fatalf("expected ExitError code 4, got %v", err)
	}
}

func TestIsWithin(t *testing.T) {
	dir := t.TempDir()
	if !isWithin(dir, dir) {
		t.Fatalf("expected same dir to be within")
	}
	if isWithin(dir, filepath.Join(dir, "..")) {
		t.Fatalf("expected outside dir to be false")
	}
}

func TestResolveExistingPathAbsolute(t *testing.T) {
	dir := t.TempDir()
	path := resolveExistingPath(filepath.Join(dir, ".."))
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
}

func pathsEquivalent(expected, actual string) bool {
	normalize := func(value string) string {
		dir := filepath.Dir(value)
		base := filepath.Base(value)
		resolvedDir := resolveExistingPath(dir)
		return filepath.Clean(filepath.Join(resolvedDir, base))
	}
	return normalize(expected) == normalize(actual)
}
