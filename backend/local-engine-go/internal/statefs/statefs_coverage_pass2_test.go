package statefs

import (
	"context"
	"errors"
	"testing"
)

func TestSanitizeSegmentFallbacks(t *testing.T) {
	if got := sanitizeSegment("тест"); got != "____" {
		t.Fatalf("expected unicode characters to be sanitized, got %q", got)
	}
	if got := sanitizeSegment("."); got != "unknown" {
		t.Fatalf("expected dot segment to fall back to unknown, got %q", got)
	}
	if got := sanitizeSegment(".."); got != "unknown" {
		t.Fatalf("expected double-dot segment to fall back to unknown, got %q", got)
	}
}

func TestManagerCloneReturnsBackendError(t *testing.T) {
	mgr := &Manager{backend: &fakeBackend{kind: "copy", cloneErr: errors.New("clone boom")}}

	if _, err := mgr.Clone(context.Background(), "src", "dest"); err == nil || err.Error() != "clone boom" {
		t.Fatalf("expected clone error, got %v", err)
	}
}

func TestRemovePathReturnsNilAfterSuccessfulRemoveAll(t *testing.T) {
	prevRemove := removeAll
	removeAll = func(path string) error { return nil }
	t.Cleanup(func() { removeAll = prevRemove })

	mgr := &Manager{backend: &fakeBackend{kind: "copy"}}
	if err := mgr.RemovePath(context.Background(), "dir"); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}
}
