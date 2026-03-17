package diff

import (
	"testing"
)

func TestParsePathScope_Valid(t *testing.T) {
	parsed, err := ParsePathScope([]string{"--from-path", "/left", "--to-path", "/right", "plan:psql", "--", "-f", "prepare.sql"})
	if err != nil {
		t.Fatalf("ParsePathScope: %v", err)
	}
	if parsed.Scope.FromPath != "/left" || parsed.Scope.ToPath != "/right" {
		t.Fatalf("unexpected scope: %+v", parsed.Scope)
	}
	if parsed.WrappedName != "plan:psql" {
		t.Fatalf("unexpected wrapped name: %q", parsed.WrappedName)
	}
	if len(parsed.WrappedArgs) != 3 || parsed.WrappedArgs[0] != "--" || parsed.WrappedArgs[1] != "-f" || parsed.WrappedArgs[2] != "prepare.sql" {
		t.Fatalf("unexpected wrapped args: %v", parsed.WrappedArgs)
	}
}

func TestParsePathScope_WithLimit(t *testing.T) {
	parsed, err := ParsePathScope([]string{"--from-path", "a", "--to-path", "b", "--limit", "5", "prepare:lb", "--", "update"})
	if err != nil {
		t.Fatalf("ParsePathScope: %v", err)
	}
	if parsed.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", parsed.Limit)
	}
	if parsed.WrappedName != "prepare:lb" {
		t.Fatalf("unexpected wrapped name: %q", parsed.WrappedName)
	}
}

func TestParsePathScope_MissingFromPath(t *testing.T) {
	_, err := ParsePathScope([]string{"--to-path", "/right", "plan:psql"})
	if err == nil || err.Error() != "diff requires --from-path and --to-path" {
		t.Fatalf("expected error about from-path, got %v", err)
	}
}

func TestParsePathScope_MissingToPath(t *testing.T) {
	_, err := ParsePathScope([]string{"--from-path", "/left", "plan:psql"})
	if err == nil || err.Error() != "diff requires --from-path and --to-path" {
		t.Fatalf("expected error about to-path, got %v", err)
	}
}

func TestParsePathScope_MissingWrappedCommand(t *testing.T) {
	_, err := ParsePathScope([]string{"--from-path", "/left", "--to-path", "/right"})
	if err == nil || err.Error() != "diff requires a wrapped command (e.g. plan:psql or prepare:lb)" {
		t.Fatalf("expected error about wrapped command, got %v", err)
	}
}

func TestParsePathScope_MissingValueFromPath(t *testing.T) {
	_, err := ParsePathScope([]string{"--from-path", "--to-path", "/right", "plan:psql"})
	if err == nil || err.Error() != "missing value for --from-path" {
		t.Fatalf("expected missing value error, got %v", err)
	}
}
