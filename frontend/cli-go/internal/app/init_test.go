package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitCreatesWorkspace(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	if err := runInit(&out, workspace, "", nil, false); err != nil {
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
	err := runInit(&out, child, "", nil, false)
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

	if err := runInit(&out, workspace, "", []string{"--dry-run"}, false); err != nil {
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
	err := runInit(&out, workspace, "", nil, false)
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
		"local",
		"--engine", absEngine,
		"--shared-cache",
	}, false)
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
	err := runInit(&out, workspace, "", []string{"local", "--engine", relEngine}, false)
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
		"local",
		"--workspace", workspace,
		"--engine", relEngine,
	}, false)
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

func TestParseInitFlagsStoreSizeRequiresImage(t *testing.T) {
	_, _, err := parseInitFlags([]string{"local", "--snapshot", "btrfs", "--store-size", "100GB"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestParseInitFlagsStoreSizeParses(t *testing.T) {
	opts, _, err := parseInitFlags([]string{"local", "--snapshot", "btrfs", "--store", "image", "--store-size", "140GB"}, "")
	if err != nil {
		t.Fatalf("parseInitFlags: %v", err)
	}
	if opts.StoreSizeGB != 140 {
		t.Fatalf("expected store size 140, got %d", opts.StoreSizeGB)
	}
	if opts.StoreType != "image" {
		t.Fatalf("expected store type image, got %q", opts.StoreType)
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
	if err := runInit(&out, workspace, "", []string{"local"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !strings.Contains(out.String(), "Workspace already initialized") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunInitRejectsMissingWorkspace(t *testing.T) {
	var out bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing")
	err := runInit(&out, missing, "", nil, false)
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
	if !isWithin(dir, filepath.Join(dir, "child")) {
		t.Fatalf("expected child dir to be within")
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

func TestRunInitHelp(t *testing.T) {
	var out bytes.Buffer
	if err := runInit(&out, t.TempDir(), "", []string{"--help"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", out.String())
	}
}

func TestRunInitExistingWorkspaceWithValidConfigDryRun(t *testing.T) {
	workspace := t.TempDir()
	marker := filepath.Join(workspace, ".sqlrs")
	if err := os.MkdirAll(marker, 0o700); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}
	configPath := filepath.Join(marker, "config.yaml")
	if err := os.WriteFile(configPath, []byte("client:\n  output: human\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"local", "--dry-run"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !strings.Contains(out.String(), "Workspace already initialized") || !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunInitForceNestedWorkspace(t *testing.T) {
	parent := t.TempDir()
	if err := os.MkdirAll(filepath.Join(parent, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("create parent marker: %v", err)
	}
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("create child: %v", err)
	}

	var out bytes.Buffer
	if err := runInit(&out, child, "", []string{"local", "--force"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !dirExists(filepath.Join(child, ".sqlrs")) {
		t.Fatalf("expected workspace marker to exist")
	}
}

func TestResolveWorkspacePathEmpty(t *testing.T) {
	if _, err := resolveWorkspacePath("", ""); err == nil {
		t.Fatalf("expected error for empty workspace path")
	}
}

func TestNormalizeEnginePathEmpty(t *testing.T) {
	dir := t.TempDir()
	if got := normalizeEnginePath("  ", dir, dir); got != "" {
		t.Fatalf("expected empty engine path, got %q", got)
	}
}

func TestResolveExistingPathRelative(t *testing.T) {
	cwd := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})

	path := resolveExistingPath("child")
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
}

func TestValidateConfigOK(t *testing.T) {
	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "config.yaml")
	if err := os.WriteFile(configPath, []byte("client:\n  output: human\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := validateConfig(configPath); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
}

func TestValidateConfigMissingFile(t *testing.T) {
	if err := validateConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatalf("expected error for missing config")
	}
}

func TestFindParentWorkspaceNoParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("create child: %v", err)
	}
	if findParentWorkspace(child) {
		t.Fatalf("expected no parent workspace")
	}
}

func TestParseStoreSizeGBValid(t *testing.T) {
	size, err := parseStoreSizeGB("100GB")
	if err != nil {
		t.Fatalf("parseStoreSizeGB: %v", err)
	}
	if size != 100 {
		t.Fatalf("expected 100, got %d", size)
	}
}

func TestParseStoreSizeGBInvalidSuffix(t *testing.T) {
	if _, err := parseStoreSizeGB("100"); err == nil {
		t.Fatalf("expected error for missing suffix")
	}
}

func TestParseStoreSizeGBInvalidValue(t *testing.T) {
	if _, err := parseStoreSizeGB("abcGB"); err == nil {
		t.Fatalf("expected error for invalid value")
	}
}

func TestParseStoreSizeGBZero(t *testing.T) {
	if _, err := parseStoreSizeGB("0GB"); err == nil {
		t.Fatalf("expected error for zero size")
	}
}

func TestParseInitFlagsStoreDeviceRequiresPath(t *testing.T) {
	_, _, err := parseInitFlags([]string{"local", "--snapshot", "btrfs", "--store", "device"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestParseInitFlagsOverlayRejectsImageStore(t *testing.T) {
	_, _, err := parseInitFlags([]string{"local", "--snapshot", "overlay", "--store", "image"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestInitWritesSnapshotBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
			return wslInitResult{UseWSL: true}, nil
		})
	}
	workspace := t.TempDir()
	var out bytes.Buffer

	if err := runInit(&out, workspace, "", []string{"local", "--snapshot", "btrfs"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "snapshot", "backend"); got != "btrfs" {
		t.Fatalf("expected snapshot.backend btrfs, got %q", got)
	}
}

func TestInitRemoteRequiresURL(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	err := runInit(&out, workspace, "", []string{"remote", "--token", "t"}, false)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestInitRemoteRequiresToken(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	err := runInit(&out, workspace, "", []string{"remote", "--url", "https://example.com"}, false)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestInitRemoteWritesProfile(t *testing.T) {
	workspace := t.TempDir()
	var out bytes.Buffer

	if err := runInit(&out, workspace, "", []string{
		"remote",
		"--url", "https://engine.example.com",
		"--token", "token-123",
	}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "defaultProfile"); got != "remote" {
		t.Fatalf("expected defaultProfile remote, got %q", got)
	}
	if got := nestedString(raw, "profiles", "remote", "mode"); got != "remote" {
		t.Fatalf("expected profiles.remote.mode remote, got %q", got)
	}
	if got := nestedString(raw, "profiles", "remote", "endpoint"); got != "https://engine.example.com" {
		t.Fatalf("expected profiles.remote.endpoint, got %q", got)
	}
	if got := nestedString(raw, "profiles", "remote", "auth", "token"); got != "token-123" {
		t.Fatalf("expected profiles.remote.auth.token, got %q", got)
	}
}

func TestPreprocessStoreArgsLongForm(t *testing.T) {
	args := []string{"--snapshot", "btrfs", "--store", "image", "/tmp/sqlrs-store", "--no-start"}
	got, err := preprocessStoreArgs(args)
	if err != nil {
		t.Fatalf("preprocessStoreArgs: %v", err)
	}
	want := []string{"--snapshot", "btrfs", "--store-type", "image", "--store-path", "/tmp/sqlrs-store", "--no-start"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args: %v", got)
	}
}

func TestPreprocessStoreArgsEqualsForm(t *testing.T) {
	args := []string{"--store=image", "/tmp/sqlrs-store"}
	got, err := preprocessStoreArgs(args)
	if err != nil {
		t.Fatalf("preprocessStoreArgs: %v", err)
	}
	want := []string{"--store-type", "image", "--store-path", "/tmp/sqlrs-store"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args: %v", got)
	}
}

func TestPreprocessStoreArgsRejectsMissingType(t *testing.T) {
	if _, err := preprocessStoreArgs([]string{"--store"}); err == nil {
		t.Fatalf("expected error for missing store type")
	}
	if _, err := preprocessStoreArgs([]string{"--store="}); err == nil {
		t.Fatalf("expected error for empty store type")
	}
}

func TestResolveStorePathUsesEnvRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", root)

	dirPath, err := resolveStorePath("dir", "")
	if err != nil {
		t.Fatalf("resolveStorePath: %v", err)
	}
	if dirPath != root {
		t.Fatalf("expected dir path %q, got %q", root, dirPath)
	}

	imagePath, err := resolveStorePath("image", "")
	if err != nil {
		t.Fatalf("resolveStorePath: %v", err)
	}
	name := "btrfs.img"
	if runtime.GOOS == "windows" {
		name = "btrfs.vhdx"
	}
	expected := filepath.Join(root, name)
	if imagePath != expected {
		t.Fatalf("expected image path %q, got %q", expected, imagePath)
	}
}

func TestResolveStorePathDeviceEmpty(t *testing.T) {
	pathValue, err := resolveStorePath("device", "")
	if err != nil {
		t.Fatalf("resolveStorePath: %v", err)
	}
	if pathValue != "" {
		t.Fatalf("expected empty path, got %q", pathValue)
	}
}

func TestParseInitFlagsStorePathRequiresType(t *testing.T) {
	_, _, err := parseInitFlags([]string{"local", "--store-path", "/tmp/store"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
	}
}

func TestParseInitFlagsRemoteRejectsLocalFlags(t *testing.T) {
	_, _, err := parseInitFlags([]string{"remote", "--url", "https://example.com", "--token", "token", "--snapshot", "btrfs"}, "")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 64 {
		t.Fatalf("expected ExitError code 64, got %v", err)
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
