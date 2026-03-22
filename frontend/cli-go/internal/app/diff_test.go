package app

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunDiff_PlanPsql(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	// Left: one file
	if err := writeFile(filepath.Join(left, "a.sql"), "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	// Right: same file, same content
	if err := writeFile(filepath.Join(right, "a.sql"), "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	var out bytes.Buffer
	args := []string{"--from-path", left, "--to-path", right, "plan:psql", "--", "-f", "a.sql"}
	err := runDiff(&out, nil, cwd, args, "human", false)
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected some output")
	}
	if !bytes.Contains(out.Bytes(), []byte("Summary:")) {
		t.Fatalf("expected Summary in output: %s", out.String())
	}
}

func TestRunDiff_UnsupportedWrappedCommand(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	args := []string{"--from-path", dir, "--to-path", dir, "run:psql", "--", "-c", "select 1"}
	err := runDiff(&out, nil, dir, args, "human", false)
	if err == nil {
		t.Fatal("expected error for run:psql")
	}
	var exitErr *ExitError
	if !isExitError(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRunDiff_InvalidScope(t *testing.T) {
	var out bytes.Buffer
	err := runDiff(&out, nil, t.TempDir(), []string{"--from-path", "/left", "plan:psql"}, "human", false)
	if err == nil {
		t.Fatal("expected error for missing --to-path")
	}
}

func TestRunDiff_RefScopePreservesSubdirCwd(t *testing.T) {
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
	sqlPath := filepath.Join(repo, "examples", "chinook", "prepare.sql")
	if err := os.MkdirAll(filepath.Dir(sqlPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(sqlPath, "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	runGit("add", "examples/chinook/prepare.sql")
	runGit("commit", "-m", "first")
	if err := writeFile(sqlPath, "select 2;\n"); err != nil {
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

	var out bytes.Buffer
	prepareArg := filepath.Join("chinook", "prepare.sql")
	err = runDiff(&out, nil, filepath.Join(repo, "examples"), []string{
		"--from-ref", string(bytes.TrimSpace(parentRef)),
		"--to-ref", string(bytes.TrimSpace(headRef)),
		"prepare:psql", "--", "-f", prepareArg,
	}, "human", false)
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Modified:\n  examples/chinook/prepare.sql")) {
		t.Fatalf("expected modified file in output, got: %s", out.String())
	}
}

func TestRunDiff_RefScopePreservesPsqlIncludeBaseDir(t *testing.T) {
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
	preparePath := filepath.Join(repo, "examples", "chinook", "prepare.sql")
	includePath := filepath.Join(repo, "examples", "chinook", "Chinook_PostgreSql.sql")
	if err := os.MkdirAll(filepath.Dir(preparePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(preparePath, "\\i 'Chinook_PostgreSql.sql'\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(includePath, "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	runGit("add", "examples/chinook/prepare.sql", "examples/chinook/Chinook_PostgreSql.sql")
	runGit("commit", "-m", "first")
	if err := writeFile(includePath, "select 2;\n"); err != nil {
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

	var out bytes.Buffer
	prepareArg := filepath.Join("chinook", "prepare.sql")
	err = runDiff(&out, nil, filepath.Join(repo, "examples"), []string{
		"--from-ref", string(bytes.TrimSpace(parentRef)),
		"--to-ref", string(bytes.TrimSpace(headRef)),
		"prepare:psql", "--", "-f", prepareArg,
	}, "human", false)
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Modified:\n  examples/chinook/Chinook_PostgreSql.sql")) {
		t.Fatalf("expected modified include file in output, got: %s", out.String())
	}
}

func TestRunDiff_RefScopeUsesRefLabelsOnErrors(t *testing.T) {
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
	if err := writeFile(filepath.Join(repo, "a.sql"), "select 1;\n"); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.sql")
	runGit("commit", "-m", "first")
	if err := writeFile(filepath.Join(repo, "a.sql"), "select 2;\n"); err != nil {
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

	var out bytes.Buffer
	err = runDiff(&out, nil, repo, []string{
		"--from-ref", string(bytes.TrimSpace(parentRef)),
		"--to-ref", string(bytes.TrimSpace(headRef)),
		"prepare:psql", "--", "-f", "missing.sql",
	}, "human", false)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if got := err.Error(); got == "" || !bytes.Contains([]byte(got), []byte("from-ref:")) || bytes.Contains([]byte(got), []byte("from-path:")) {
		t.Fatalf("expected from-ref label, got %v", err)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func isExitError(err error, out **ExitError) bool {
	return errors.As(err, out)
}
