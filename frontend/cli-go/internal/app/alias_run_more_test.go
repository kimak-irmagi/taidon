package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRunAliasArgsAdditionalBranches(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		_, showHelp, err := parseRunAliasArgs([]string{"--help"}, true)
		if err != nil || !showHelp {
			t.Fatalf("expected help, err=%v showHelp=%v", err, showHelp)
		}
	})

	t.Run("unicode dash", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"smoke", "—instance", "dev"}, true)
		if err == nil || !strings.Contains(err.Error(), "Unicode dash") {
			t.Fatalf("expected unicode dash hint, got %v", err)
		}
	})

	t.Run("missing instance value", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"smoke", "--instance"}, true)
		if err == nil || !strings.Contains(err.Error(), "Missing value for --instance") {
			t.Fatalf("expected missing instance error, got %v", err)
		}
	})

	t.Run("missing instance equals value", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"smoke", "--instance="}, true)
		if err == nil || !strings.Contains(err.Error(), "Missing value for --instance") {
			t.Fatalf("expected missing instance error, got %v", err)
		}
	})

	t.Run("reject second ref", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"smoke", "other", "--instance", "dev"}, true)
		if err == nil || !strings.Contains(err.Error(), "exactly one alias ref") {
			t.Fatalf("expected second ref error, got %v", err)
		}
	})
}

func TestResolveRunAliasPathAdditionalValidation(t *testing.T) {
	t.Run("requires base path", func(t *testing.T) {
		_, err := resolveRunAliasPath("", "", "smoke")
		if err == nil || !strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("expected base path error, got %v", err)
		}
	})

	t.Run("rejects empty exact ref", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := resolveRunAliasPath(workspace, workspace, ".")
		if err == nil || !strings.Contains(err.Error(), "run alias ref is empty") {
			t.Fatalf("expected empty exact-ref error, got %v", err)
		}
	})
}

func TestLoadRunAliasReadAndYAMLErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		_, err := loadRunAlias(filepath.Join(t.TempDir(), "missing.run.s9s.yaml"))
		if err == nil {
			t.Fatalf("expected read error")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("expected not-exist error, got %v", err)
		}
	})

	t.Run("yaml error", func(t *testing.T) {
		path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "kind: [\n")
		_, err := loadRunAlias(path)
		if err == nil || !strings.Contains(err.Error(), "read run alias") {
			t.Fatalf("expected yaml error, got %v", err)
		}
	})
}
