package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveXDG(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	dirs, err := resolveXDG()
	if err != nil {
		t.Fatalf("resolveXDG: %v", err)
	}
	if !filepath.IsAbs(dirs.ConfigDir) || !filepath.IsAbs(dirs.StateDir) || !filepath.IsAbs(dirs.CacheDir) {
		t.Fatalf("expected absolute dirs, got %+v", dirs)
	}
}

func TestResolveWindows(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(root, "appdata"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))

	dirs, err := resolveWindows()
	if err != nil {
		t.Fatalf("resolveWindows: %v", err)
	}
	if filepath.Base(dirs.ConfigDir) != "sqlrs" || filepath.Base(dirs.StateDir) != "sqlrs" || filepath.Base(dirs.CacheDir) != "sqlrs" {
		t.Fatalf("unexpected dirs: %+v", dirs)
	}
}

func TestResolveDarwin(t *testing.T) {
	dirs, err := resolveDarwin()
	if err != nil {
		t.Fatalf("resolveDarwin: %v", err)
	}
	if filepath.Base(dirs.ConfigDir) != "config" || filepath.Base(dirs.StateDir) != "state" || filepath.Base(dirs.CacheDir) != "cache" {
		t.Fatalf("unexpected dirs: %+v", dirs)
	}
}

func TestFindProjectConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".sqlrs")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	child := filepath.Join(root, "child", "deep")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	found, err := FindProjectConfig(child)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if found != configPath {
		t.Fatalf("expected %q, got %q", configPath, found)
	}
}

func TestGetenvOr(t *testing.T) {
	t.Setenv("SQLRS_TEST_ENV", "value")
	if got := getenvOr("SQLRS_TEST_ENV", "fallback"); got != "value" {
		t.Fatalf("expected env value, got %q", got)
	}
	if got := getenvOr("SQLRS_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if !fileExists(path) {
		t.Fatalf("expected file to exist")
	}
	if fileExists(filepath.Join(dir, "missing")) {
		t.Fatalf("expected missing to be false")
	}
}
