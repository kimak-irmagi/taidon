package config

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManagerReturnsLoadOverridesError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := NewManager(Options{StateStoreRoot: root}); err == nil {
		t.Fatalf("expected loadOverrides error")
	}
}

func TestConfigSetReturnsMarshalError(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewManager(Options{StateStoreRoot: root})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Set("features.nan", math.NaN()); err == nil || !strings.Contains(err.Error(), "unsupported value") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestConfigRemoveReturnsMarshalError(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewManager(Options{StateStoreRoot: root})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.overrides = map[string]any{
		"features": map[string]any{
			"flag": true,
		},
		"broken": math.NaN(),
	}

	if _, err := mgr.Remove("features.flag"); err == nil || !strings.Contains(err.Error(), "unsupported value") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}
