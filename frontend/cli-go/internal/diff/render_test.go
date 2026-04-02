package diff

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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
	RenderHuman(&buf, result, Scope{Kind: ScopeKindPath, FromPath: fromDir, ToPath: toDir}, "plan:psql",
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
	RenderHuman(&buf, result, Scope{Kind: ScopeKindPath, FromPath: fromDir, ToPath: toDir}, "plan:psql",
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
	if err := RenderJSON(&buf, result, Scope{Kind: ScopeKindPath, FromPath: "", ToPath: toDir}, "plan:psql",
		Options{IncludeContent: true}, Context{}, Context{Root: toDir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"content":"body\n`) && !strings.Contains(buf.String(), `"content":"body`) {
		t.Fatalf("expected content field in JSON: %s", buf.String())
	}
}

func TestRenderHuman_RefHeaderOmitsDefaultWorktreeMode(t *testing.T) {
	var buf bytes.Buffer
	RenderHuman(
		&buf,
		DiffResult{},
		Scope{Kind: ScopeKindRef, FromRef: "HEAD~1", ToRef: "HEAD", RefMode: "worktree"},
		"plan:psql -- -f a.sql",
		Options{},
		Context{},
		Context{},
	)
	if strings.Contains(buf.String(), "--ref-mode worktree") {
		t.Fatalf("default worktree mode should be omitted from header: %s", buf.String())
	}
}

func TestRenderHuman_RefHeaderShowsExplicitBlobMode(t *testing.T) {
	var buf bytes.Buffer
	RenderHuman(
		&buf,
		DiffResult{},
		Scope{Kind: ScopeKindRef, FromRef: "HEAD~1", ToRef: "HEAD", RefMode: "blob"},
		"plan:psql -- -f a.sql",
		Options{},
		Context{},
		Context{},
	)
	if !strings.Contains(buf.String(), "--ref-mode blob") {
		t.Fatalf("explicit blob mode should be shown in header: %s", buf.String())
	}
}

func TestRenderHuman_LimitTruncatesListButKeepsSummary(t *testing.T) {
	result := DiffResult{
		Added: []FileEntry{
			{Path: "a.sql", Hash: "h1"},
			{Path: "b.sql", Hash: "h2"},
		},
	}
	var buf bytes.Buffer
	RenderHuman(
		&buf,
		result,
		Scope{Kind: ScopeKindPath, FromPath: "left", ToPath: "right"},
		"plan:psql -- -f a.sql",
		Options{Limit: 1},
		Context{},
		Context{},
	)
	out := buf.String()
	if !strings.Contains(out, "  a.sql\n") {
		t.Fatalf("expected first entry in output: %s", out)
	}
	if strings.Contains(out, "  b.sql\n") {
		t.Fatalf("did not expect truncated entry in output: %s", out)
	}
	if !strings.Contains(out, "... (1 more)") {
		t.Fatalf("expected truncation marker in output: %s", out)
	}
	if !strings.Contains(out, "Summary: 2 added, 0 modified, 0 removed") {
		t.Fatalf("expected full summary in output: %s", out)
	}
}

func TestRenderJSON_LimitAndRefScope(t *testing.T) {
	result := DiffResult{
		Added: []FileEntry{
			{Path: "a.sql", Hash: "h1"},
			{Path: "b.sql", Hash: "h2"},
		},
	}
	var buf bytes.Buffer
	err := RenderJSON(
		&buf,
		result,
		Scope{
			Kind:            ScopeKindRef,
			FromRef:         "HEAD~1",
			ToRef:           "HEAD",
			RefMode:         "blob",
			RefKeepWorktree: false,
		},
		"plan:psql -- -f a.sql",
		Options{Limit: 1},
		Context{},
		Context{},
	)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, buf.String())
	}
	if out.Scope.Mode != string(ScopeKindRef) || out.Scope.FromRef != "HEAD~1" || out.Scope.ToRef != "HEAD" || out.Scope.RefMode != "blob" {
		t.Fatalf("unexpected scope: %+v", out.Scope)
	}
	if len(out.Added) != 1 || out.Added[0].Path != "a.sql" {
		t.Fatalf("expected truncated added entries, got %+v", out.Added)
	}
	if out.Summary.Added != 2 || out.Summary.Modified != 0 || out.Summary.Removed != 0 {
		t.Fatalf("expected full summary, got %+v", out.Summary)
	}
}

func TestRenderJSON_IncludeContentFromBlobContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	emptyTemplate := t.TempDir()
	repo := t.TempDir()
	initCmd := exec.Command("git", "-C", repo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "a.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.sql")
	runGit("commit", "-m", "first")

	var buf bytes.Buffer
	err := RenderJSON(
		&buf,
		DiffResult{Added: []FileEntry{{Path: "a.sql", Hash: "h1"}}},
		Scope{Kind: ScopeKindRef, FromRef: "HEAD", ToRef: "HEAD", RefMode: "blob"},
		"plan:psql -- -f a.sql",
		Options{IncludeContent: true},
		Context{Root: repo, GitRef: "HEAD"},
		Context{Root: repo, GitRef: "HEAD"},
	)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"content":"select 1`) {
		t.Fatalf("expected blob-backed content in JSON: %s", buf.String())
	}
}
