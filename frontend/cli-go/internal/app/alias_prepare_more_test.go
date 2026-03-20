package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePrepareAliasArgsAdditionalBranches(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		_, showHelp, err := parsePrepareAliasArgs([]string{"--help"})
		if err != nil || !showHelp {
			t.Fatalf("expected help, err=%v showHelp=%v", err, showHelp)
		}
	})

	t.Run("watch specified", func(t *testing.T) {
		invocation, showHelp, err := parsePrepareAliasArgs([]string{"--watch", "chinook"})
		if err != nil || showHelp {
			t.Fatalf("parsePrepareAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.Ref != "chinook" || !invocation.Watch || !invocation.WatchSpecified {
			t.Fatalf("unexpected invocation: %+v", invocation)
		}
	})

	t.Run("reject unknown option", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"chinook", "--bad"})
		if err == nil || !strings.Contains(err.Error(), "unknown prepare alias option") {
			t.Fatalf("expected unknown option error, got %v", err)
		}
	})

	t.Run("reject second ref", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"chinook", "other"})
		if err == nil || !strings.Contains(err.Error(), "exactly one alias ref") {
			t.Fatalf("expected second ref error, got %v", err)
		}
	})
}

func TestParsePlanAliasArgsAdditionalBranches(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		_, showHelp, err := parsePlanAliasArgs([]string{"-h"})
		if err != nil || !showHelp {
			t.Fatalf("expected help, err=%v showHelp=%v", err, showHelp)
		}
	})

	t.Run("reject unknown option", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"chinook", "--bad"})
		if err == nil || !strings.Contains(err.Error(), "unknown plan alias option") {
			t.Fatalf("expected unknown option error, got %v", err)
		}
	})

	t.Run("reject second ref", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"chinook", "other"})
		if err == nil || !strings.Contains(err.Error(), "exactly one alias ref") {
			t.Fatalf("expected second ref error, got %v", err)
		}
	})
}

func TestResolvePrepareAliasPathAdditionalValidation(t *testing.T) {
	t.Run("requires base path", func(t *testing.T) {
		_, err := resolvePrepareAliasPath("", "", "chinook")
		if err == nil || !strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("expected base path error, got %v", err)
		}
	})

	t.Run("rejects empty exact ref", func(t *testing.T) {
		workspace := t.TempDir()
		_, err := resolvePrepareAliasPath(workspace, workspace, ".")
		if err == nil || !strings.Contains(err.Error(), "prepare alias ref is empty") {
			t.Fatalf("expected empty exact-ref error, got %v", err)
		}
	})
}

func TestLoadPrepareAliasReadAndYAMLErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		_, err := loadPrepareAlias(filepath.Join(t.TempDir(), "missing.prep.s9s.yaml"))
		if err == nil {
			t.Fatalf("expected read error")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("expected not-exist error, got %v", err)
		}
	})

	t.Run("yaml error", func(t *testing.T) {
		path := writePrepareAliasFile(t, t.TempDir(), "broken.prep.s9s.yaml", "kind: [\n")
		_, err := loadPrepareAlias(path)
		if err == nil || !strings.Contains(err.Error(), "read prepare alias") {
			t.Fatalf("expected yaml error, got %v", err)
		}
	})
}

func TestBuildPrepareAliasCommandArgsWatchAndImage(t *testing.T) {
	args := buildPrepareAliasCommandArgs(
		prepareAlias{
			Image: " postgres:17 ",
			Args:  []string{"-f", "prepare.sql"},
		},
		prepareAliasInvocation{
			Watch:          true,
			WatchSpecified: true,
		},
	)
	if got := strings.Join(args, "|"); got != "--watch|--image|postgres:17|--|-f|prepare.sql" {
		t.Fatalf("args = %q", got)
	}
}
