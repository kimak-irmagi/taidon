package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
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

	t.Run("trimmed instance value", func(t *testing.T) {
		invocation, showHelp, err := parseRunAliasArgs([]string{"smoke", "--instance", " dev "}, true)
		if err != nil || showHelp {
			t.Fatalf("parseRunAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.InstanceRef != "dev" {
			t.Fatalf("unexpected instance: %q", invocation.InstanceRef)
		}
	})

	t.Run("missing instance equals value", func(t *testing.T) {
		_, _, err := parseRunAliasArgs([]string{"smoke", "--instance="}, true)
		if err == nil || !strings.Contains(err.Error(), "Missing value for --instance") {
			t.Fatalf("expected missing instance error, got %v", err)
		}
	})

	t.Run("instance equals value", func(t *testing.T) {
		invocation, showHelp, err := parseRunAliasArgs([]string{"smoke", "--instance=dev"}, true)
		if err != nil || showHelp {
			t.Fatalf("parseRunAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.InstanceRef != "dev" {
			t.Fatalf("unexpected instance: %q", invocation.InstanceRef)
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

	t.Run("uses workspace root when cwd is empty", func(t *testing.T) {
		workspace := t.TempDir()
		writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
		got, err := resolveRunAliasPath(workspace, "", "smoke")
		if err != nil {
			t.Fatalf("resolveRunAliasPath: %v", err)
		}
		want := filepath.Join(workspace, "smoke.run.s9s.yaml")
		if got != want {
			t.Fatalf("unexpected path: got %q want %q", got, want)
		}
	})

	t.Run("uses cwd when workspace root is empty", func(t *testing.T) {
		workspace := t.TempDir()
		writeRunAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
		got, err := resolveRunAliasPath("", workspace, "smoke")
		if err != nil {
			t.Fatalf("resolveRunAliasPath: %v", err)
		}
		want := filepath.Join(workspace, "smoke.run.s9s.yaml")
		if got != want {
			t.Fatalf("unexpected path: got %q want %q", got, want)
		}
	})
}

func TestLoadRunAliasReadAndYAMLErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: filepath.Join(t.TempDir(), "missing.run.s9s.yaml")})
		if err == nil {
			t.Fatalf("expected read error")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("expected not-exist error, got %v", err)
		}
	})

	t.Run("yaml error", func(t *testing.T) {
		path := writeRunAliasFile(t, t.TempDir(), "broken.run.s9s.yaml", "kind: [\n")
		_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: path})
		if err == nil || !strings.Contains(err.Error(), "read run alias") {
			t.Fatalf("expected yaml error, got %v", err)
		}
	})
}
