package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
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

	t.Run("no-watch specified", func(t *testing.T) {
		invocation, showHelp, err := parsePrepareAliasArgs([]string{"--no-watch", "chinook"})
		if err != nil || showHelp {
			t.Fatalf("parsePrepareAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.Ref != "chinook" || invocation.Watch || !invocation.WatchSpecified {
			t.Fatalf("unexpected invocation: %+v", invocation)
		}
	})

	t.Run("ref options", func(t *testing.T) {
		invocation, showHelp, err := parsePrepareAliasArgs([]string{"--ref", "HEAD~1", "--ref-mode", "blob", "chinook"})
		if err != nil || showHelp {
			t.Fatalf("parsePrepareAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.Ref != "chinook" || invocation.GitRef != "HEAD~1" || invocation.RefMode != "blob" {
			t.Fatalf("unexpected invocation: %+v", invocation)
		}
	})

	t.Run("ref defaults to worktree", func(t *testing.T) {
		invocation, showHelp, err := parsePrepareAliasArgs([]string{"--ref", "origin/main", "chinook"})
		if err != nil || showHelp {
			t.Fatalf("parsePrepareAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.Ref != "chinook" || invocation.GitRef != "origin/main" || invocation.RefMode != "worktree" {
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

	t.Run("reject keep worktree without ref", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref-keep-worktree", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-keep-worktree requires --ref") {
			t.Fatalf("expected keep-worktree error, got %v", err)
		}
	})

	t.Run("reject missing ref value", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --ref") {
			t.Fatalf("expected missing ref error, got %v", err)
		}
	})

	t.Run("reject empty ref value", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref", " ", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --ref") {
			t.Fatalf("expected empty ref error, got %v", err)
		}
	})

	t.Run("reject ref mode without ref", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref-mode", "blob", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-mode requires --ref") {
			t.Fatalf("expected ref-mode requires ref error, got %v", err)
		}
	})

	t.Run("reject missing ref mode value", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref", "HEAD", "--ref-mode"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --ref-mode") {
			t.Fatalf("expected missing ref-mode error, got %v", err)
		}
	})

	t.Run("reject keep worktree with blob", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref", "HEAD", "--ref-mode", "blob", "--ref-keep-worktree", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-keep-worktree is only valid with --ref-mode worktree") {
			t.Fatalf("expected keep-worktree/blob error, got %v", err)
		}
	})

	t.Run("reject no-watch with ref", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref", "HEAD", "--no-watch", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--no-watch is not supported with --ref") {
			t.Fatalf("expected no-watch/ref error, got %v", err)
		}
	})

	t.Run("reject bad ref mode", func(t *testing.T) {
		_, _, err := parsePrepareAliasArgs([]string{"--ref", "HEAD", "--ref-mode", "bad", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-mode \"bad\" is not supported") {
			t.Fatalf("expected bad ref-mode error, got %v", err)
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

	t.Run("reject watch flags", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--watch", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "plan does not support --watch/--no-watch") {
			t.Fatalf("expected watch rejection, got %v", err)
		}
	})

	t.Run("reject ref mode without ref", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--ref-mode", "blob", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-mode requires --ref") {
			t.Fatalf("expected ref-mode requires ref error, got %v", err)
		}
	})

	t.Run("reject keep worktree without ref", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--ref-keep-worktree", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-keep-worktree requires --ref") {
			t.Fatalf("expected keep-worktree requires ref error, got %v", err)
		}
	})

	t.Run("reject missing ref mode value", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--ref", "HEAD", "--ref-mode"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --ref-mode") {
			t.Fatalf("expected missing ref-mode error, got %v", err)
		}
	})

	t.Run("reject missing ref value", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--ref"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --ref") {
			t.Fatalf("expected missing ref error, got %v", err)
		}
	})

	t.Run("reject bad ref mode", func(t *testing.T) {
		_, _, err := parsePlanAliasArgs([]string{"--ref", "HEAD", "--ref-mode", "bad", "chinook"})
		if err == nil || !strings.Contains(err.Error(), "--ref-mode \"bad\" is not supported") {
			t.Fatalf("expected bad ref-mode error, got %v", err)
		}
	})

	t.Run("ref options", func(t *testing.T) {
		invocation, showHelp, err := parsePlanAliasArgs([]string{"--ref", "origin/main", "--ref-keep-worktree", "chinook"})
		if err != nil || showHelp {
			t.Fatalf("parsePlanAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if invocation.Ref != "chinook" {
			t.Fatalf("ref = %q", invocation.Ref)
		}
		if invocation.GitRef != "origin/main" || invocation.RefMode != "worktree" || !invocation.RefKeepWorktree {
			t.Fatalf("unexpected invocation: %+v", invocation)
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

	t.Run("uses cwd when workspace root is empty", func(t *testing.T) {
		cwd := t.TempDir()
		expected := writePrepareAliasFile(t, cwd, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
		path, err := resolvePrepareAliasPath("", cwd, "chinook")
		if err != nil {
			t.Fatalf("resolvePrepareAliasPath: %v", err)
		}
		if path != expected {
			t.Fatalf("path = %q, want %q", path, expected)
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
		_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassPrepare, Path: filepath.Join(t.TempDir(), "missing.prep.s9s.yaml")})
		if err == nil {
			t.Fatalf("expected read error")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("expected not-exist error, got %v", err)
		}
	})

	t.Run("yaml error", func(t *testing.T) {
		path := writePrepareAliasFile(t, t.TempDir(), "broken.prep.s9s.yaml", "kind: [\n")
		_, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassPrepare, Path: path})
		if err == nil || !strings.Contains(err.Error(), "read prepare alias") {
			t.Fatalf("expected yaml error, got %v", err)
		}
	})
}

func TestBuildPrepareAliasCommandArgsWatchAndImage(t *testing.T) {
	args := buildPrepareAliasCommandArgs(
		aliaspkg.Definition{
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

func TestBuildPrepareAliasCommandArgsRefOptions(t *testing.T) {
	args := buildPrepareAliasCommandArgs(
		aliaspkg.Definition{
			Image: " postgres:17 ",
			Args:  []string{"-f", "prepare.sql"},
		},
		prepareAliasInvocation{
			GitRef:         "HEAD~1",
			RefMode:        "blob",
			Watch:          false,
			WatchSpecified: true,
		},
	)
	if got := strings.Join(args, "|"); got != "--ref|HEAD~1|--ref-mode|blob|--image|postgres:17|--|-f|prepare.sql" {
		t.Fatalf("args = %q", got)
	}
}

func TestBuildPlanAliasCommandArgsRefKeepWorktree(t *testing.T) {
	args := buildPlanAliasCommandArgs(
		aliaspkg.Definition{
			Image: " postgres:17 ",
			Args:  []string{"update"},
		},
		planAliasInvocation{
			GitRef:          "origin/main",
			RefMode:         "worktree",
			RefKeepWorktree: true,
		},
	)
	if got := strings.Join(args, "|"); got != "--ref|origin/main|--ref-keep-worktree|--image|postgres:17|--|update" {
		t.Fatalf("args = %q", got)
	}
}
