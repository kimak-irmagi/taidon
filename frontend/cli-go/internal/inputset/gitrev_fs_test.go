package inputset

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitTestRepo(t *testing.T) string {
	t.Helper()
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
	return repo
}

func TestGitRevFileSystem_ReadFileAndStat(t *testing.T) {
	repo := initGitTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
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
	if got := HashContent(b); len(got) != 64 {
		t.Fatalf("hash length: %d", len(got))
	}
}

func TestGitRevFileSystem_ReadDir(t *testing.T) {
	repo := initGitTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
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
	rootEntries, err := fs.ReadDir(repo)
	if err != nil {
		t.Fatalf("ReadDir repo root: %v", err)
	}
	if len(rootEntries) != 1 || !rootEntries[0].IsDir() || rootEntries[0].Name() != "sub" {
		t.Fatalf("unexpected root entries: %+v", rootEntries)
	}
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

func TestGitRevFileSystem_RootStatAndMissingPath(t *testing.T) {
	repo := initGitTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "hello.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "hello.sql")
	if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dir", "nested.sql"), []byte("select 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "dir")
	runGit("commit", "-m", "first")

	fs := NewGitRevFileSystem(repo, "HEAD")
	fi, err := fs.Stat(repo)
	if err != nil {
		t.Fatalf("Stat repo root: %v", err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected repo root to be dir, got %#v", fi)
	}
	fi, err = fs.Stat(filepath.Join(repo, "dir"))
	if err != nil {
		t.Fatalf("Stat nested dir: %v", err)
	}
	if !fi.IsDir() || fi.Name() != "dir" {
		t.Fatalf("expected nested dir stat, got %#v", fi)
	}
	_, err = fs.Stat(filepath.Join(repo, "missing.sql"))
	if err == nil {
		t.Fatal("expected missing path error")
	}
	_, err = fs.ReadDir(filepath.Join(repo, "hello.sql"))
	if err == nil {
		t.Fatal("expected ReadDir error for file path")
	}
}

func TestGitRevFileSystem_HelperBranches(t *testing.T) {
	repo := initGitTestRepo(t)
	outside := t.TempDir()
	fsys := NewGitRevFileSystem(" "+repo+" ", " HEAD ")

	t.Run("absToRel rejects empty repo or ref", func(t *testing.T) {
		if _, err := (&GitRevFileSystem{}).absToRel(filepath.Join(repo, "a.sql")); err == nil {
			t.Fatal("expected empty repo/ref error")
		}
	})

	t.Run("absToRel rejects outside path", func(t *testing.T) {
		if _, err := fsys.absToRel(filepath.Join(outside, "a.sql")); err == nil {
			t.Fatal("expected outside repository error")
		}
	})

	t.Run("absToRel returns empty rel for repo root", func(t *testing.T) {
		rel, err := fsys.absToRel(repo)
		if err != nil {
			t.Fatalf("absToRel(repo): %v", err)
		}
		if rel != "" {
			t.Fatalf("expected empty rel for repo root, got %q", rel)
		}
	})

	t.Run("normalizeGitRel filters traversal", func(t *testing.T) {
		cases := map[string]string{
			"./a.sql":      "a.sql",
			"..":           "",
			"../a.sql":     "",
			"dir/../a.sql": "a.sql",
			"a/../../b":    "",
			" a.sql ":      "a.sql",
		}
		for in, want := range cases {
			if got := normalizeGitRel(in); got != want {
				t.Fatalf("normalizeGitRel(%q) = %q, want %q", in, got, want)
			}
		}
	})

	t.Run("parseLsTreeDirEntries ignores malformed and unsupported records", func(t *testing.T) {
		out := []byte("100644 blob abc123\ta.sql\x00bad-record\x00160000 commit def456\tsubmodule\x00040000 tree xyz789\tdir\x00100644 blob xyz789\tdir/b.sql\x00")
		entries := parseLsTreeDirEntries(out)
		if len(entries) != 2 || entries[0].Name() != "a.sql" || entries[0].IsDir() || entries[1].Name() != "dir" || !entries[1].IsDir() {
			t.Fatalf("unexpected parsed entries: %+v", entries)
		}
	})

	t.Run("parseLsTreeDirEntries skips empty and incomplete metadata", func(t *testing.T) {
		out := []byte("\x00100644\ta.sql\x00")
		entries := parseLsTreeDirEntries(out)
		if len(entries) != 0 {
			t.Fatalf("expected no entries, got %+v", entries)
		}
	})
}

func TestGitRevFileSystem_ErrorBranches(t *testing.T) {
	repo := initGitTestRepo(t)
	fsys := NewGitRevFileSystem(repo, "HEAD")

	if err := os.WriteFile(filepath.Join(repo, "hello.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("add", "hello.sql")
	runGit("commit", "-m", "first")

	t.Run("ReadFile rejects repo root path", func(t *testing.T) {
		if _, err := fsys.ReadFile(repo); err == nil {
			t.Fatal("expected invalid root object path error")
		}
	})

	t.Run("ReadFile reports git show error for missing file", func(t *testing.T) {
		if _, err := fsys.ReadFile(filepath.Join(repo, "missing.sql")); err == nil || !strings.Contains(err.Error(), "git show HEAD:missing.sql") {
			t.Fatalf("expected git show error, got %v", err)
		}
	})

	t.Run("Stat invalid without repo or ref", func(t *testing.T) {
		if _, err := (&GitRevFileSystem{}).Stat("anything"); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("expected fs.ErrInvalid, got %v", err)
		}
	})

	t.Run("Stat repo root rejects invalid ref", func(t *testing.T) {
		if _, err := NewGitRevFileSystem(repo, "missing-ref").Stat(repo); err == nil {
			t.Fatal("expected invalid ref error")
		}
	})

	t.Run("Stat treats gitlink entries as unsupported", func(t *testing.T) {
		cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v", err)
		}
		oid := strings.TrimSpace(string(out))
		runGit("update-index", "--add", "--cacheinfo", "160000,"+oid+",gitlink")
		runGit("commit", "-m", "gitlink")
		if _, err := fsys.Stat(filepath.Join(repo, "gitlink")); err == nil {
			t.Fatal("expected gitlink stat to be treated as missing")
		}
	})

	t.Run("ReadDir reports git ls-tree error for missing path", func(t *testing.T) {
		if _, err := fsys.ReadDir(filepath.Join(repo, "missing-dir")); err == nil || !strings.Contains(err.Error(), "git ls-tree HEAD:missing-dir") {
			t.Fatalf("expected git ls-tree error, got %v", err)
		}
	})

	t.Run("ReadDir rejects outside path", func(t *testing.T) {
		if _, err := fsys.ReadDir(filepath.Join(t.TempDir(), "outside")); err == nil {
			t.Fatal("expected outside repository error")
		}
	})

	t.Run("gitCatFileSize reports bad spec", func(t *testing.T) {
		if _, err := gitCatFileSize(repo, "HEAD:missing.sql"); err == nil {
			t.Fatal("expected git cat-file size error")
		}
	})

	t.Run("git file info helpers", func(t *testing.T) {
		dirInfo := &gitFileInfo{name: "dir", isDir: true}
		fileInfo := &gitFileInfo{name: "file.sql", size: 7}
		if dirInfo.Name() != "dir" || !dirInfo.IsDir() || dirInfo.Mode()&fs.ModeDir == 0 || !dirInfo.ModTime().IsZero() || dirInfo.Sys() != nil {
			t.Fatalf("unexpected dir info: %#v", dirInfo)
		}
		if fileInfo.Name() != "file.sql" || fileInfo.IsDir() || fileInfo.Size() != 7 || fileInfo.Mode() != 0o644 || fileInfo.Sys() != nil {
			t.Fatalf("unexpected file info: %#v", fileInfo)
		}
		entry := gitDirEntry{info: dirInfo}
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("entry.Info: %v", err)
		}
		if entry.Name() != "dir" || !entry.IsDir() || entry.Type()&fs.ModeDir == 0 || info.Name() != "dir" {
			t.Fatalf("unexpected dir entry: %#v info=%#v", entry, info)
		}
	})
}
