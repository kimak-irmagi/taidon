package diff

import (
	"testing"
)

func TestParseDiffScope_PathValid(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-path", "/left", "--to-path", "/right", "plan:psql", "--", "-f", "prepare.sql"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if parsed.Scope.Kind != ScopeKindPath || parsed.Scope.FromPath != "/left" || parsed.Scope.ToPath != "/right" {
		t.Fatalf("unexpected scope: %+v", parsed.Scope)
	}
	if parsed.WrappedName != "plan:psql" {
		t.Fatalf("unexpected wrapped name: %q", parsed.WrappedName)
	}
	if len(parsed.WrappedArgs) != 3 || parsed.WrappedArgs[0] != "--" || parsed.WrappedArgs[1] != "-f" || parsed.WrappedArgs[2] != "prepare.sql" {
		t.Fatalf("unexpected wrapped args: %v", parsed.WrappedArgs)
	}
}

func TestParseDiffScope_RefValid(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-ref", "main", "--to-ref", "HEAD", "plan:psql", "--", "-f", "a.sql"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if parsed.Scope.Kind != ScopeKindRef || parsed.Scope.FromRef != "main" || parsed.Scope.ToRef != "HEAD" {
		t.Fatalf("unexpected scope: %+v", parsed.Scope)
	}
	if parsed.Scope.RefMode != "worktree" {
		t.Fatalf("expected default ref-mode worktree, got %q", parsed.Scope.RefMode)
	}
}

func TestParseDiffScope_RefKeepWorktree(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-mode", "worktree", "--ref-keep-worktree", "prepare:lb", "--", "x"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if !parsed.Scope.RefKeepWorktree {
		t.Fatal("expected RefKeepWorktree")
	}
}

func TestParseDiffScope_RefModeBlobExplicit(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-mode", "blob", "plan:psql", "--", "-f", "x.sql"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if parsed.Scope.RefMode != "blob" {
		t.Fatalf("expected ref-mode blob, got %q", parsed.Scope.RefMode)
	}
}

func TestParseDiffScope_RefKeepWorktreeUsesDefaultWorktreeMode(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-keep-worktree", "plan:psql"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if parsed.Scope.RefMode != "worktree" || !parsed.Scope.RefKeepWorktree {
		t.Fatalf("expected default worktree mode with keep flag, got %+v", parsed.Scope)
	}
}

func TestParseDiffScope_RefKeepWorktreeRejectedForBlobMode(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-mode", "blob", "--ref-keep-worktree", "plan:psql"})
	if err == nil {
		t.Fatal("expected error when --ref-keep-worktree is used with blob mode")
	}
}

func TestParseDiffScope_RefModeInvalid(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-mode", "nosuch", "plan:psql"})
	if err == nil {
		t.Fatal("expected error for invalid ref-mode")
	}
}

func TestParseDiffScope_MissingRefModeValue(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-ref", "a", "--to-ref", "b", "--ref-mode"})
	if err == nil || err.Error() != "missing value for --ref-mode" {
		t.Fatalf("expected missing ref-mode value error, got %v", err)
	}
}

func TestParseDiffScope_MixedPathAndRef(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "/a", "--to-ref", "HEAD", "plan:psql"})
	if err == nil {
		t.Fatal("expected error when mixing path and ref")
	}
}

func TestParseDiffScope_WithLimit(t *testing.T) {
	parsed, err := ParseDiffScope([]string{"--from-path", "a", "--to-path", "b", "--limit", "5", "prepare:lb", "--", "update"})
	if err != nil {
		t.Fatalf("ParseDiffScope: %v", err)
	}
	if parsed.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", parsed.Limit)
	}
}

func TestParseDiffScope_InvalidLimit(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "a", "--to-path", "b", "--limit", "nope", "prepare:lb"})
	if err == nil || err.Error() != "--limit must be a non-negative integer" {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
}

func TestParseDiffScope_NegativeLimit(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "a", "--to-path", "b", "--limit", "-1", "prepare:lb"})
	if err == nil || err.Error() != "--limit must be a non-negative integer" {
		t.Fatalf("expected negative limit error, got %v", err)
	}
}

func TestParseDiffScope_MissingFromPath(t *testing.T) {
	_, err := ParseDiffScope([]string{"--to-path", "/right", "plan:psql"})
	if err == nil || err.Error() != "diff requires both --from-path and --to-path, or both --from-ref and --to-ref" {
		t.Fatalf("expected paired path/ref error, got %v", err)
	}
}

func TestParseDiffScope_MissingToPath(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "/left", "plan:psql"})
	if err == nil || err.Error() != "diff requires both --from-path and --to-path, or both --from-ref and --to-ref" {
		t.Fatalf("expected paired path/ref error, got %v", err)
	}
}

func TestParseDiffScope_MissingWrappedCommand(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "/left", "--to-path", "/right"})
	if err == nil || err.Error() != "diff requires a wrapped command (e.g. plan:psql or prepare:lb)" {
		t.Fatalf("expected error about wrapped command, got %v", err)
	}
}

func TestParseDiffScope_MissingScope(t *testing.T) {
	_, err := ParseDiffScope([]string{"plan:psql", "--", "-f", "a.sql"})
	if err == nil || err.Error() != "diff requires a scope: --from-path/--to-path or --from-ref/--to-ref" {
		t.Fatalf("expected missing scope error, got %v", err)
	}
}

func TestParseDiffScope_MissingValueFromPath(t *testing.T) {
	_, err := ParseDiffScope([]string{"--from-path", "--to-path", "/right", "plan:psql"})
	if err == nil || err.Error() != "missing value for --from-path" {
		t.Fatalf("expected missing value error, got %v", err)
	}
}
