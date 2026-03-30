package inputset

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRevFileSystem_ReadFileAndStat(t *testing.T) {
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
	p := filepath.Join(repo, "hello.sql")
	if err := os.WriteFile(p, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "hello.sql")
	runGit("commit", "-m", "first")

	fs := NewGitRevFileSystem(repo, "HEAD")
	abs := filepath.Join(repo, "hello.sql")
	fi, err := fs.Stat(abs)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.IsDir() || fi.Name() != "hello.sql" {
		t.Fatalf("unexpected stat: %#v", fi)
	}
	b, err := fs.ReadFile(abs)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "select 1;\n" {
		t.Fatalf("content: %q", b)
	}
	oid, err := fs.BlobOID(abs)
	if err != nil {
		t.Fatalf("BlobOID: %v", err)
	}
	if len(oid) != 40 {
		t.Fatalf("blob oid length: %d %q", len(oid), oid)
	}
	cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD:hello.sql")
	want, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	if strings.TrimSpace(string(want)) != oid {
		t.Fatalf("BlobOID %q != rev-parse %q", oid, strings.TrimSpace(string(want)))
	}
}

func TestGitRevFileSystem_ReadDir(t *testing.T) {
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
	sub := filepath.Join(repo, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.sql"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.sql"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "sub")
	runGit("commit", "-m", "first")

	fs := NewGitRevFileSystem(repo, "HEAD")
	entries, err := fs.ReadDir(sub)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "a.sql") || !strings.Contains(got, "b.sql") {
		t.Fatalf("names: %v", names)
	}
}
