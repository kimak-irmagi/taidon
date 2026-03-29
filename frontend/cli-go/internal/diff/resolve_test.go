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

func TestResolveScope_RefWorktreesPreserveRelativeCwd(t *testing.T) {
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
	sqlPath := filepath.Join(repo, "examples", "prepare.sql")
	if err := os.MkdirAll(filepath.Dir(sqlPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqlPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "examples/prepare.sql")
	runGit("commit", "-m", "first")
	if err := os.WriteFile(sqlPath, []byte("select 2;\n"), 0o600); err != nil {
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
	scope := Scope{
		Kind:    ScopeKindRef,
		FromRef: strings.TrimSpace(string(parentRef)),
		ToRef:   strings.TrimSpace(string(headRef)),
		RefMode: "worktree",
	}
	cwd := filepath.Join(repo, "examples")
	fromCtx, toCtx, cleanup, err := ResolveScope(scope, cwd)
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup for ref mode")
	}
	defer func() { _ = cleanup() }()
	if filepath.Base(fromCtx.BaseDir) != "examples" || filepath.Base(toCtx.BaseDir) != "examples" {
		t.Fatalf("expected mirrored cwd in contexts, got from=%+v to=%+v", fromCtx, toCtx)
	}
	fromList, err := BuildPsqlFileList(fromCtx, []string{"-f", "prepare.sql"})
	if err != nil {
		t.Fatalf("from: %v", err)
	}
	toList, err := BuildPsqlFileList(toCtx, []string{"-f", "prepare.sql"})
	if err != nil {
		t.Fatalf("to: %v", err)
	}
	result := Compare(fromList, toList, Options{})
	if len(result.Modified) != 1 || result.Modified[0].Path != "examples/prepare.sql" {
		t.Fatalf("expected modified examples/prepare.sql, got %+v", result)
	}
}

func TestResolveScope_RefWorktreesCanonicalizesSymlinkedCwd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	emptyTemplate := t.TempDir()
	realRepo := t.TempDir()
	linkRepo := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(realRepo, linkRepo); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", realRepo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	initCmd := exec.Command("git", "-C", realRepo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init skipped (need writable temp; run tests outside sandbox): %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")
	sqlPath := filepath.Join(realRepo, "a.sql")
	if err := os.WriteFile(sqlPath, []byte("v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.sql")
	runGit("commit", "-m", "first")
	if err := os.WriteFile(sqlPath, []byte("v2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("commit", "-am", "second")
	headRef, err := exec.Command("git", "-C", realRepo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	parentRef, err := exec.Command("git", "-C", realRepo, "rev-parse", "HEAD^").Output()
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{
		Kind:    ScopeKindRef,
		FromRef: strings.TrimSpace(string(parentRef)),
		ToRef:   strings.TrimSpace(string(headRef)),
		RefMode: "worktree",
	}
	fromCtx, toCtx, cleanup, err := ResolveScope(scope, linkRepo)
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup for ref mode")
	}
	defer func() { _ = cleanup() }()
	if _, err := BuildPsqlFileList(fromCtx, []string{"-f", "a.sql"}); err != nil {
		t.Fatalf("from: %v", err)
	}
	if _, err := BuildPsqlFileList(toCtx, []string{"-f", "a.sql"}); err != nil {
		t.Fatalf("to: %v", err)
	}
}

func TestResolveScope_RefBlobNoWorktree(t *testing.T) {
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
		RefMode: "blob",
	}
	fromCtx, toCtx, cleanup, err := ResolveScope(scope, repo)
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	if cleanup != nil {
<<<<<<< Updated upstream
		t.Fatal("blob ref mode should not return cleanup")
	}
	if fromCtx.GitRef != parent || toCtx.GitRef != head {
		t.Fatalf("expected GitRef on contexts, from=%+v to=%+v", fromCtx, toCtx)
=======
		t.Fatal("expected nil cleanup for blob ref mode")
	}
	if fromCtx.GitRef != parent || toCtx.GitRef != head {
		t.Fatalf("unexpected GitRef: from=%q to=%q", fromCtx.GitRef, toCtx.GitRef)
>>>>>>> Stashed changes
	}
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
