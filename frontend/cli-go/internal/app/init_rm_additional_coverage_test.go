package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
)

func TestReadConfigMapAdditionalErrors(t *testing.T) {
	t.Run("missing-file", func(t *testing.T) {
		if _, err := readConfigMap(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
			t.Fatalf("expected missing file error")
		}
	})

	t.Run("invalid-yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("root: ["), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if _, err := readConfigMap(path); err == nil {
			t.Fatalf("expected yaml parse error")
		}
	})
}

func TestRunRmParseErrorPath(t *testing.T) {
	err := runRm(&bytes.Buffer{}, cli.RmOptions{}, []string{"--unknown", "deadbeef"}, "human")
	if err == nil || !strings.Contains(err.Error(), "Invalid arguments") {
		t.Fatalf("expected parse error, got %v", err)
	}
}
