package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig(t *testing.T) {
	root := t.TempDir()

	rootConfig := filepath.Join(root, ".sqlrs", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(rootConfig), 0o700); err != nil {
		t.Fatalf("mkdir root config: %v", err)
	}
	if err := os.WriteFile(rootConfig, []byte("x: 1\n"), 0o600); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	found, err := FindProjectConfig(nested)
	if err != nil {
		t.Fatalf("find config: %v", err)
	}
	if found != rootConfig {
		t.Fatalf("expected %q, got %q", rootConfig, found)
	}

	nearer := filepath.Join(root, "a", ".sqlrs", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(nearer), 0o700); err != nil {
		t.Fatalf("mkdir nearer config: %v", err)
	}
	if err := os.WriteFile(nearer, []byte("x: 2\n"), 0o600); err != nil {
		t.Fatalf("write nearer config: %v", err)
	}

	found, err = FindProjectConfig(nested)
	if err != nil {
		t.Fatalf("find config (nearest): %v", err)
	}
	if found != nearer {
		t.Fatalf("expected %q, got %q", nearer, found)
	}
}
