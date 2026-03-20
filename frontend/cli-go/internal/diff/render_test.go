package diff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderHuman_IncludeContent(t *testing.T) {
	fromDir := t.TempDir()
	toDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fromDir, "only-from.sql"), []byte("from;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toDir, "only-to.sql"), []byte("to line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := DiffResult{
		Added:    []FileEntry{{Path: "only-to.sql", Hash: "h1"}},
		Modified: []FileEntry{{Path: "x.sql", Hash: "h2"}},
		Removed:  []FileEntry{{Path: "only-from.sql", Hash: "h3"}},
	}
	// modified exists on "to" only for snippet purposes
	if err := os.WriteFile(filepath.Join(toDir, "x.sql"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	RenderHuman(&buf, result, PathScope{FromPath: fromDir, ToPath: toDir}, "plan:psql",
		Options{IncludeContent: true}, Context{Root: fromDir}, Context{Root: toDir})
	out := buf.String()
	if !strings.Contains(out, "only-to.sql") || !strings.Contains(out, "to line") {
		t.Fatalf("expected added snippet in output:\n%s", out)
	}
	if !strings.Contains(out, "only-from.sql") || !strings.Contains(out, "from;") {
		t.Fatalf("expected removed snippet in output:\n%s", out)
	}
	if !strings.Contains(out, "x.sql") || !strings.Contains(out, "new") {
		t.Fatalf("expected modified snippet in output:\n%s", out)
	}
}

func TestRenderHuman_IncludeContentOff(t *testing.T) {
	fromDir := t.TempDir()
	toDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(toDir, "a.sql"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := DiffResult{Added: []FileEntry{{Path: "a.sql", Hash: "h"}}}
	var buf bytes.Buffer
	RenderHuman(&buf, result, PathScope{FromPath: fromDir, ToPath: toDir}, "plan:psql",
		Options{IncludeContent: false}, Context{Root: fromDir}, Context{Root: toDir})
	if strings.Contains(buf.String(), "secret") {
		t.Fatalf("did not expect content when IncludeContent false:\n%s", buf.String())
	}
}

func TestRenderJSON_IncludeContent(t *testing.T) {
	toDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(toDir, "b.sql"), []byte("body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := DiffResult{Added: []FileEntry{{Path: "b.sql", Hash: "h"}}}
	var buf bytes.Buffer
	if err := RenderJSON(&buf, result, PathScope{ToPath: toDir}, "plan:psql",
		Options{IncludeContent: true}, Context{}, Context{Root: toDir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"content":"body\n`) && !strings.Contains(buf.String(), `"content":"body`) {
		t.Fatalf("expected content field in JSON: %s", buf.String())
	}
}
