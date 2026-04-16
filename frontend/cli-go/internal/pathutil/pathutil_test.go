package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCanonicalizeBoundaryPath(t *testing.T) {
	root := t.TempDir()
	if got := CanonicalizeBoundaryPath(""); got != "." {
		t.Fatalf("CanonicalizeBoundaryPath(empty) = %q, want .", got)
	}
	if got := CanonicalizeBoundaryPath(root); got == "" {
		t.Fatal("CanonicalizeBoundaryPath(existing) returned empty string")
	}

	missing := filepath.Join(root, "nested", "missing.sql")
	got := CanonicalizeBoundaryPath(missing)
	if !strings.HasSuffix(filepath.ToSlash(got), "nested/missing.sql") {
		t.Fatalf("CanonicalizeBoundaryPath(missing) = %q", got)
	}
}

func TestCanonicalizeBoundaryPathResolvesSymlinkedParent(t *testing.T) {
	realRoot := t.TempDir()
	linkDir := t.TempDir()
	linkRoot := filepath.Join(linkDir, "workspace-link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	got := CanonicalizeBoundaryPath(filepath.Join(linkRoot, "nested", "query.sql"))
	wantRoot := CanonicalizeBoundaryPath(realRoot)
	rel, err := filepath.Rel(wantRoot, got)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q): %v", wantRoot, got, err)
	}
	if rel != filepath.Join("nested", "query.sql") {
		t.Fatalf("CanonicalizeBoundaryPath(symlinked missing) relative path = %q", rel)
	}
}

func TestIsWithin(t *testing.T) {
	root := t.TempDir()
	if !IsWithin(root, root) {
		t.Fatalf("expected root to be within itself")
	}
	if !IsWithin(root, filepath.Join(root, "nested")) {
		t.Fatalf("expected nested path to be within root")
	}
	if IsWithin(root, filepath.Join(filepath.Dir(root), "outside")) {
		t.Fatalf("expected outside path to fail boundary check")
	}
	if IsWithin(root, "relative") {
		t.Fatalf("expected relative target to fail boundary check")
	}
}

func TestSameLocalPath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "query.sql")
	withSlashes := filepath.ToSlash(nested)
	if !SameLocalPath(nested, withSlashes) {
		t.Fatalf("SameLocalPath(%q, %q) = false", nested, withSlashes)
	}

	if runtime.GOOS == "windows" && !SameLocalPath(`C:\Temp\Query.sql`, `c:/temp/query.sql`) {
		t.Fatalf("expected Windows path comparison to be case-insensitive")
	}
}

func TestSameLocalPathResolvesSymlinkedParent(t *testing.T) {
	realRoot := t.TempDir()
	linkDir := t.TempDir()
	linkRoot := filepath.Join(linkDir, "workspace-link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	if !SameLocalPath(linkRoot, realRoot) {
		t.Fatalf("expected symlink and real root to compare equal")
	}
}
