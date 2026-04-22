package refctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
	"github.com/sqlrs/cli/internal/pathutil"
)

func TestResolveBlobContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initRefctxTestRepo(t)
	workspaceRoot := filepath.Join(repo, "workspace")
	cwd := filepath.Join(workspaceRoot, "app")

	ctx, err := Resolve(workspaceRoot, cwd, head, "blob", false)
	if err != nil {
		t.Fatalf("Resolve(blob): %v", err)
	}
	if ctx.GitRef != head {
		t.Fatalf("GitRef = %q, want %q", ctx.GitRef, head)
	}
	if ctx.RefMode != "blob" {
		t.Fatalf("RefMode = %q, want blob", ctx.RefMode)
	}
	if !pathutil.SameLocalPath(ctx.RepoRoot, repo) {
		t.Fatalf("RepoRoot = %q, want path equivalent to %q", ctx.RepoRoot, repo)
	}
	if !pathutil.SameLocalPath(ctx.WorkspaceRoot, workspaceRoot) {
		t.Fatalf("WorkspaceRoot = %q, want path equivalent to %q", ctx.WorkspaceRoot, workspaceRoot)
	}
	if !pathutil.SameLocalPath(ctx.BaseDir, cwd) {
		t.Fatalf("BaseDir = %q, want path equivalent to %q", ctx.BaseDir, cwd)
	}
	if _, ok := ctx.FileSystem.(*inputset.GitRevFileSystem); !ok {
		t.Fatalf("FileSystem = %T, want *inputset.GitRevFileSystem", ctx.FileSystem)
	}
	if err := ctx.Cleanup(); err != nil {
		t.Fatalf("Cleanup(blob): %v", err)
	}
}

func TestResolveWorktreeContextAndCleanup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initRefctxTestRepo(t)
	workspaceRoot := filepath.Join(repo, "workspace")
	cwd := filepath.Join(workspaceRoot, "app")

	ctx, err := Resolve(workspaceRoot, cwd, head, "worktree", false)
	if err != nil {
		t.Fatalf("Resolve(worktree): %v", err)
	}
	if ctx.RefMode != "worktree" {
		t.Fatalf("RefMode = %q, want worktree", ctx.RefMode)
	}
	if _, ok := ctx.FileSystem.(inputset.OSFileSystem); !ok {
		t.Fatalf("FileSystem = %T, want inputset.OSFileSystem", ctx.FileSystem)
	}
	if !pathutil.IsWithin(ctx.RepoRoot, ctx.BaseDir) {
		t.Fatalf("BaseDir = %q, want path under RepoRoot %q", ctx.BaseDir, ctx.RepoRoot)
	}
	if _, err := os.Stat(ctx.BaseDir); err != nil {
		t.Fatalf("stat BaseDir before cleanup: %v", err)
	}
	if err := ctx.Cleanup(); err != nil {
		t.Fatalf("Cleanup(worktree): %v", err)
	}
	if _, err := os.Stat(ctx.RepoRoot); !os.IsNotExist(err) {
		t.Fatalf("expected RepoRoot removed after cleanup, got %v", err)
	}
}

func TestResolveRejectsInvalidInputs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initRefctxTestRepo(t)
	workspaceRoot := filepath.Join(repo, "workspace")
	cwd := filepath.Join(workspaceRoot, "app")
	outside := t.TempDir()
	missingAtRef := filepath.Join(workspaceRoot, "later")
	if err := os.MkdirAll(missingAtRef, 0o700); err != nil {
		t.Fatalf("mkdir missingAtRef: %v", err)
	}

	cases := []struct {
		name          string
		workspaceRoot string
		cwd           string
		gitRef        string
		refMode       string
		want          string
	}{
		{
			name:    "missing ref",
			cwd:     cwd,
			want:    "git ref is required",
			refMode: "blob",
		},
		{
			name:    "bad mode",
			cwd:     cwd,
			gitRef:  head,
			refMode: "nope",
			want:    "unsupported ref mode",
		},
		{
			name:          "cwd outside repo",
			workspaceRoot: workspaceRoot,
			cwd:           outside,
			gitRef:        head,
			refMode:       "blob",
			want:          "not a git repository",
		},
		{
			name:          "workspace outside repo",
			workspaceRoot: outside,
			cwd:           cwd,
			gitRef:        head,
			refMode:       "blob",
			want:          "resolve workspace root",
		},
		{
			name:          "projected cwd missing at ref",
			workspaceRoot: workspaceRoot,
			cwd:           missingAtRef,
			gitRef:        head,
			refMode:       "blob",
			want:          "projected cwd missing at ref",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.workspaceRoot, tc.cwd, tc.gitRef, tc.refMode, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Resolve(%q, %q, %q, %q) error = %v, want substring %q", tc.workspaceRoot, tc.cwd, tc.gitRef, tc.refMode, err, tc.want)
			}
		})
	}
}

func TestResolveDefaultsAndKeepWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initRefctxTestRepo(t)
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})

	ctx, err := Resolve("", "", head, "", true)
	if err != nil {
		t.Fatalf("Resolve(defaults): %v", err)
	}
	if ctx.RefMode != "worktree" {
		t.Fatalf("RefMode = %q, want worktree", ctx.RefMode)
	}
	if ctx.WorkspaceRoot != "" {
		t.Fatalf("WorkspaceRoot = %q, want empty", ctx.WorkspaceRoot)
	}
	if !pathutil.SameLocalPath(ctx.BaseDir, ctx.RepoRoot) {
		t.Fatalf("BaseDir = %q, want path equivalent to RepoRoot %q", ctx.BaseDir, ctx.RepoRoot)
	}
	if err := ctx.Cleanup(); err != nil {
		t.Fatalf("Cleanup(keep worktree): %v", err)
	}
	if _, err := os.Stat(ctx.RepoRoot); err != nil {
		t.Fatalf("expected kept worktree to remain on disk: %v", err)
	}
	if err := gitWorktreeRemove(repo, ctx.RepoRoot); err != nil {
		t.Fatalf("gitWorktreeRemove(kept worktree): %v", err)
	}
}

func TestRefCtxProjectedCWDParityForPlanPrepare(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initRefctxTestRepo(t)
	workspaceRoot := filepath.Join(repo, "workspace")
	cwd := filepath.Join(workspaceRoot, "app")

	for _, mode := range []string{"worktree", "blob"} {
		t.Run(mode, func(t *testing.T) {
			ctx, err := Resolve(workspaceRoot, cwd, head, mode, false)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", mode, err)
			}
			t.Cleanup(func() {
				if err := ctx.Cleanup(); err != nil {
					t.Fatalf("Cleanup(%s): %v", mode, err)
				}
			})

			baseRel, err := filepath.Rel(ctx.RepoRoot, ctx.BaseDir)
			if err != nil {
				t.Fatalf("filepath.Rel(BaseDir): %v", err)
			}
			if filepath.Clean(baseRel) != filepath.Join("workspace", "app") {
				t.Fatalf("BaseDir rel = %q, want %q", baseRel, filepath.Join("workspace", "app"))
			}

			workspaceRel, err := filepath.Rel(ctx.RepoRoot, ctx.WorkspaceRoot)
			if err != nil {
				t.Fatalf("filepath.Rel(WorkspaceRoot): %v", err)
			}
			if filepath.Clean(workspaceRel) != "workspace" {
				t.Fatalf("WorkspaceRoot rel = %q, want %q", workspaceRel, "workspace")
			}

			data, err := ctx.FileSystem.ReadFile(filepath.Join(ctx.BaseDir, "query.sql"))
			if err != nil {
				t.Fatalf("ReadFile(query.sql): %v", err)
			}
			normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
			if normalized != "select 1;\n" {
				t.Fatalf("query.sql contents = %q, want %q", string(data), "select 1;\n")
			}
		})
	}
}

func TestResolveRejectsWhenGitUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := Resolve("", ".", "HEAD", "blob", false); err == nil || !strings.Contains(err.Error(), "git: git not found in PATH") {
		t.Fatalf("expected git unavailable error, got %v", err)
	}
}

func TestResolveWorktreeProjectedCwdMissingAtRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, firstRef := initRefctxTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	laterDir := filepath.Join(repo, "workspace", "later")
	if err := os.MkdirAll(laterDir, 0o700); err != nil {
		t.Fatalf("mkdir later dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(laterDir, "query.sql"), []byte("select 2;\n"), 0o600); err != nil {
		t.Fatalf("write later query: %v", err)
	}
	runGit("add", "workspace")
	runGit("commit", "-m", "add later cwd")

	_, err := Resolve(filepath.Join(repo, "workspace"), laterDir, firstRef, "worktree", false)
	if err == nil || !strings.Contains(err.Error(), "projected cwd missing at ref") {
		t.Fatalf("expected projected cwd missing error, got %v", err)
	}
}

func TestRefctxHelpers(t *testing.T) {
	t.Run("ensureProjectedDir", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "file.txt")
		if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if err := ensureProjectedDir(inputset.OSFileSystem{}, root, "dir"); err != nil {
			t.Fatalf("ensureProjectedDir(dir): %v", err)
		}
		if err := ensureProjectedDir(inputset.OSFileSystem{}, file, "not dir"); err == nil || !strings.Contains(err.Error(), "not dir") {
			t.Fatalf("expected file path error, got %v", err)
		}
		if err := ensureProjectedDir(inputset.OSFileSystem{}, filepath.Join(root, "missing"), "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("expected missing path error, got %v", err)
		}
	})

	t.Run("pathWithinRepo", func(t *testing.T) {
		repo := t.TempDir()
		child := filepath.Join(repo, "nested", "query.sql")
		outside := t.TempDir()
		parent := filepath.Dir(repo)

		if rel, err := pathWithinRepo(repo, repo); err != nil || rel != "." {
			t.Fatalf("pathWithinRepo(repo, repo) = %q, %v", rel, err)
		}
		if rel, err := pathWithinRepo(repo, child); err != nil || rel != filepath.Join("nested", "query.sql") {
			t.Fatalf("pathWithinRepo child = %q, %v", rel, err)
		}
		if _, err := pathWithinRepo(repo, outside); err == nil || !strings.Contains(err.Error(), "outside repository root") {
			t.Fatalf("expected outside error, got %v", err)
		}
		if _, err := pathWithinRepo(repo, parent); err == nil || !strings.Contains(err.Error(), "outside repository root") {
			t.Fatalf("expected parent dir outside error, got %v", err)
		}
	})

	t.Run("optionalPathWithinRepo", func(t *testing.T) {
		repo := t.TempDir()
		if rel, ok, err := optionalPathWithinRepo(repo, ""); err != nil || ok || rel != "" {
			t.Fatalf("optionalPathWithinRepo empty = %q, %v, %v", rel, ok, err)
		}
		child := filepath.Join(repo, "nested")
		if rel, ok, err := optionalPathWithinRepo(repo, child); err != nil || !ok || rel != "nested" {
			t.Fatalf("optionalPathWithinRepo child = %q, %v, %v", rel, ok, err)
		}
	})

	t.Run("gitUnavailable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if got := gitUnavailable(); !strings.Contains(got, "git not found in PATH") {
			t.Fatalf("gitUnavailable() = %q", got)
		}
	})

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	t.Run("git helpers", func(t *testing.T) {
		repo, head := initRefctxTestRepo(t)

		top, err := gitTopLevel(filepath.Join(repo, "workspace", "app"))
		if err != nil {
			t.Fatalf("gitTopLevel: %v", err)
		}
		if !pathutil.SameLocalPath(top, repo) {
			t.Fatalf("gitTopLevel = %q, want path equivalent to %q", top, repo)
		}
		if _, err := gitTopLevel(t.TempDir()); err == nil {
			t.Fatalf("expected gitTopLevel failure outside repo")
		}

		worktree := filepath.Join(t.TempDir(), "wt")
		if err := gitWorktreeAddDetach(repo, worktree, head); err != nil {
			t.Fatalf("gitWorktreeAddDetach: %v", err)
		}
		if _, err := os.Stat(worktree); err != nil {
			t.Fatalf("stat worktree: %v", err)
		}
		if err := gitWorktreeRemove(repo, worktree); err != nil {
			t.Fatalf("gitWorktreeRemove: %v", err)
		}
		if _, err := os.Stat(worktree); !os.IsNotExist(err) {
			t.Fatalf("expected removed worktree, got %v", err)
		}
	})
}

func TestGitHelpersGenericFailures(t *testing.T) {
	fakeBin := t.TempDir()
	fakeGit := filepath.Join(fakeBin, "git")
	if runtime.GOOS == "windows" {
		fakeGit += ".cmd"
		if err := os.WriteFile(fakeGit, []byte("@echo off\r\nexit /b 1\r\n"), 0o600); err != nil {
			t.Fatalf("write fake git.cmd: %v", err)
		}
	} else {
		if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
	}
	t.Setenv("PATH", fakeBin)

	if _, err := gitTopLevel("ignored"); err == nil || !strings.Contains(err.Error(), "not a git repository (or git failed)") {
		t.Fatalf("expected generic gitTopLevel error, got %v", err)
	}
	if err := gitWorktreeAddDetach("repo", "path", "ref"); err == nil || !strings.Contains(err.Error(), "add detached worktree for ref") {
		t.Fatalf("expected gitWorktreeAddDetach error, got %v", err)
	}
	if err := gitWorktreeRemove("repo", "path"); err == nil || !strings.Contains(err.Error(), "remove detached worktree path") {
		t.Fatalf("expected gitWorktreeRemove error, got %v", err)
	}
}

func initRefctxTestRepo(t *testing.T) (string, string) {
	t.Helper()

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
		t.Skipf("git init skipped: %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")

	appDir := filepath.Join(repo, "workspace", "app")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "query.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write query.sql: %v", err)
	}
	runGit("add", "workspace")
	runGit("commit", "-m", "initial")

	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return repo, strings.TrimSpace(string(out))
}
