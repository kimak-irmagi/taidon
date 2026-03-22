package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveScope_RefWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	emptyTemplate := t.TempDir()
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	initCmd := exec.Command("git", "-C", repo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init skipped (need writable temp; run tests outside sandbox): %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")
	sqlA := filepath.Join(repo, "a.sql")
	if err := os.WriteFile(sqlA, []byte("v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.sql")
	runGit("commit", "-m", "first")
	if err := os.WriteFile(sqlA, []byte("v2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("commit", "-am", "second")
	headRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	parentRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD^").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headRef))
	parent := strings.TrimSpace(string(parentRef))
	scope := Scope{
		Kind:    ScopeKindRef,
		FromRef: parent,
		ToRef:   head,
		RefMode: "worktree",
	}
	fromCtx, toCtx, cleanup, err := ResolveScope(scope, repo)
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup for ref mode")
	}
	defer func() { _ = cleanup() }()
	fromList, err := BuildPsqlFileList(fromCtx, []string{"-f", "a.sql"})
	if err != nil {
		t.Fatalf("from: %v", err)
	}
	toList, err := BuildPsqlFileList(toCtx, []string{"-f", "a.sql"})
	if err != nil {
		t.Fatalf("to: %v", err)
	}
	result := Compare(fromList, toList, Options{})
	if len(result.Modified) != 1 || result.Modified[0].Path != "a.sql" {
		t.Fatalf("expected modified a.sql, got %+v", result)
	}
}
