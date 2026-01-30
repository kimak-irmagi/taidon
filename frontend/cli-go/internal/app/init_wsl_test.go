package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitWSLHappyAutoDistro(t *testing.T) {
	workspace := t.TempDir()
	var captured wslInitOptions
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		captured = opts
		return wslInitResult{
			UseWSL:     true,
			Distro:     "Ubuntu",
			StateDir:   "/var/lib/sqlrs",
			EnginePath: "/opt/sqlrs/sqlrs-engine",
			StorePath:  "C:\\sqlrs\\store\\btrfs.vhdx",
		}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	if !captured.Enable {
		t.Fatalf("expected Enable true")
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
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
	if got := nestedString(raw, "engine", "wsl", "enginePath"); got != "/opt/sqlrs/sqlrs-engine" {
		t.Fatalf("expected enginePath, got %q", got)
	}
	if got := nestedString(raw, "engine", "storePath"); got != "C:\\sqlrs\\store\\btrfs.vhdx" {
		t.Fatalf("expected storePath, got %q", got)
	}
}

func TestInitWSLUsesExplicitDistro(t *testing.T) {
	workspace := t.TempDir()
	var captured wslInitOptions
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		captured = opts
		return wslInitResult{UseWSL: true, Distro: opts.Distro}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl", "--distro", "Debian"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	if captured.Distro != "Debian" {
		t.Fatalf("expected Debian, got %q", captured.Distro)
	}
}

func TestInitWSLNoStartSkipsWSLStart(t *testing.T) {
	workspace := t.TempDir()
	var captured wslInitOptions
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		captured = opts
		return wslInitResult{UseWSL: true}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl", "--no-start"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !captured.NoStart {
		t.Fatalf("expected NoStart true")
	}
}

func TestInitWSLPassesStoreFlags(t *testing.T) {
	workspace := t.TempDir()
	var captured wslInitOptions
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		captured = opts
		return wslInitResult{UseWSL: true}, nil
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl", "--store-size", "120GB", "--reinit"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if captured.StoreSizeGB != 120 {
		t.Fatalf("expected store size 120, got %d", captured.StoreSizeGB)
	}
	if !captured.Reinit {
		t.Fatalf("expected reinit true")
	}
}

func TestInitWSLRequireFailsWhenWSLMissing(t *testing.T) {
	workspace := t.TempDir()
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		return wslInitResult{}, errTestWSL
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl", "--require"}, false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInitWSLWarnsAndFallbackWhenWSLMissing(t *testing.T) {
	workspace := t.TempDir()
	withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
		return wslInitResult{UseWSL: false, Warning: "WSL unavailable"}, nil
	})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
		os.Stderr = oldStderr
	})

	var out bytes.Buffer
	if err := runInit(&out, workspace, "", []string{"--wsl"}, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if !strings.Contains(string(data), "WSL unavailable") {
		t.Fatalf("expected warning, got %q", string(data))
	}

	configPath := filepath.Join(workspace, ".sqlrs", "config.yaml")
	raw := loadConfigMap(t, configPath)
	if got := nestedString(raw, "engine", "wsl", "mode"); got != "auto" {
		t.Fatalf("expected mode auto, got %q", got)
	}
}

func withInitWSLStub(t *testing.T, fn func(opts wslInitOptions) (wslInitResult, error)) {
	t.Helper()
	prev := initWSLFn
	initWSLFn = fn
	t.Cleanup(func() {
		initWSLFn = prev
	})
}

func loadConfigMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	return raw
}

func nestedString(root map[string]any, keys ...string) string {
	current := any(root)
	for _, key := range keys {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = next[key]
	}
	value, ok := current.(string)
	if !ok {
		return ""
	}
	return value
}

var errTestWSL = errInitTest("wsl init failed")

type errInitTest string

func (e errInitTest) Error() string {
	return string(e)
}
