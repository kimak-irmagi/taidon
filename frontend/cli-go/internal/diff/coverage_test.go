package diff

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/pathutil"
)

func TestDiffCoverageHelpers(t *testing.T) {
	t.Run("parse scope branches", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
			want string
		}{
			{name: "missing from ref value", args: []string{"--from-ref", "--to-ref", "b", "plan:psql"}, want: "missing value for --from-ref"},
			{name: "empty from path value", args: []string{"--from-path", "", "--to-path", "b", "plan:psql"}, want: "--from-path value is empty"},
			{name: "empty to path value", args: []string{"--from-path", "a", "--to-path", "", "plan:psql"}, want: "--to-path value is empty"},
			{name: "empty from ref value", args: []string{"--from-ref", "", "--to-ref", "b", "plan:psql"}, want: "--from-ref value is empty"},
			{name: "empty to ref value", args: []string{"--from-ref", "a", "--to-ref", "", "plan:psql"}, want: "--to-ref value is empty"},
			{name: "missing to ref value", args: []string{"--from-ref", "a", "--to-ref"}, want: "missing value for --to-ref"},
			{name: "missing ref mode value", args: []string{"--from-ref", "a", "--to-ref", "b", "--ref-mode"}, want: "missing value for --ref-mode"},
			{name: "missing limit value", args: []string{"--from-path", "a", "--to-path", "b", "--limit"}, want: "missing value for --limit"},
			{name: "invalid limit", args: []string{"--from-path", "a", "--to-path", "b", "--limit", "nope", "plan:psql"}, want: "--limit must be a non-negative integer"},
			{name: "ref keep worktree rejected for blob", args: []string{"--from-ref", "a", "--to-ref", "b", "--ref-mode", "blob", "--ref-keep-worktree", "plan:psql"}, want: "diff: --ref-keep-worktree is only valid with --ref-mode worktree"},
			{name: "scope required", args: []string{"plan:psql"}, want: "diff requires a scope: --from-path/--to-path or --from-ref/--to-ref"},
			{name: "from ref only", args: []string{"--from-ref", "a", "plan:psql"}, want: "diff requires both --from-path and --to-path, or both --from-ref and --to-ref"},
			{name: "to ref only", args: []string{"--to-ref", "b", "plan:psql"}, want: "diff requires both --from-path and --to-path, or both --from-ref and --to-ref"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ParseDiffScope(tc.args)
				if err == nil || err.Error() != tc.want {
					t.Fatalf("ParseDiffScope(%v) = %v, want %q", tc.args, err, tc.want)
				}
			})
		}

		parsed, err := ParseDiffScope([]string{"--from-path", "left", "--to-path", "right", "--include-content", "plan:psql", "--", "-f", "a.sql"})
		if err != nil {
			t.Fatalf("ParseDiffScope(include-content): %v", err)
		}
		if !parsed.IncludeContent || parsed.WrappedName != "plan:psql" {
			t.Fatalf("unexpected parsed scope: %+v", parsed)
		}

		from := "left"
		to := "right"
		if _, err := finishParseDiffScope(&from, &to, nil, nil, "", false, Options{}, nil); err == nil || !strings.Contains(err.Error(), "wrapped command") {
			t.Fatalf("expected wrapped command error, got %v", err)
		}
	})

	t.Run("resolve scope and path helpers", func(t *testing.T) {
		workspace := t.TempDir()
		if fromCtx, toCtx, cleanup, err := ResolveScope(Scope{Kind: ScopeKindPath, FromPath: "from", ToPath: "to"}, ""); err != nil {
			t.Fatalf("ResolveScope path default cwd: %v", err)
		} else if cleanup != nil || filepath.Base(fromCtx.Root) != "from" || filepath.Base(toCtx.Root) != "to" {
			t.Fatalf("unexpected default cwd path scope: from=%+v to=%+v cleanup=%v", fromCtx, toCtx, cleanup != nil)
		}
		fromCtx, toCtx, cleanup, err := ResolveScope(Scope{Kind: ScopeKindPath, FromPath: "from", ToPath: "to"}, workspace)
		if err != nil {
			t.Fatalf("ResolveScope path: %v", err)
		}
		if cleanup != nil {
			t.Fatalf("expected no cleanup for path scope")
		}
		if want := filepath.Join(workspace, "from"); !pathutil.SameLocalPath(fromCtx.Root, want) || !pathutil.SameLocalPath(fromCtx.BaseDir, want) {
			t.Fatalf("unexpected from context: %+v", fromCtx)
		}
		if want := filepath.Join(workspace, "to"); !pathutil.SameLocalPath(toCtx.Root, want) || !pathutil.SameLocalPath(toCtx.BaseDir, want) {
			t.Fatalf("unexpected to context: %+v", toCtx)
		}

		if _, _, err := resolvePathScopeStrings("", "right", workspace); err == nil || !strings.Contains(err.Error(), "from-path") {
			t.Fatalf("expected from-path error, got %v", err)
		}
		if _, _, err := resolvePathScopeStrings("left", "", workspace); err == nil || !strings.Contains(err.Error(), "to-path") {
			t.Fatalf("expected to-path error, got %v", err)
		}
		if _, err := absPathInCwd("", workspace); err == nil || !strings.Contains(err.Error(), "path is empty") {
			t.Fatalf("expected empty path error, got %v", err)
		}
		if got, err := absPathInCwd("rel.sql", ""); err != nil {
			t.Fatalf("absPathInCwd relative: %v", err)
		} else if want, _ := filepath.Abs("rel.sql"); !pathutil.SameLocalPath(got, want) {
			t.Fatalf("absPathInCwd relative = %q, want %q", got, want)
		}
		if got, err := absPathInCwd(filepath.Join(workspace, "abs.sql"), workspace); err != nil {
			t.Fatalf("absPathInCwd absolute: %v", err)
		} else if !pathutil.SameLocalPath(got, filepath.Join(workspace, "abs.sql")) {
			t.Fatalf("absPathInCwd absolute = %q", got)
		}

		if _, _, _, err := ResolveScope(Scope{Kind: "bogus"}, workspace); err == nil || !strings.Contains(err.Error(), "unknown scope kind") {
			t.Fatalf("expected unknown scope kind error, got %v", err)
		}

		if rel, err := cwdWithinRepo(workspace, filepath.Join(workspace, "nested")); err != nil {
			t.Fatalf("cwdWithinRepo success: %v", err)
		} else if rel != "nested" {
			t.Fatalf("cwdWithinRepo success = %q", rel)
		}
		if runtime.GOOS == "windows" {
			if _, err := cwdWithinRepo(`C:\repo`, `D:\cwd`); err == nil {
				t.Fatalf("expected cross-volume cwdWithinRepo error")
			}
		}

		if got := formatDiffCommandLine(Scope{Kind: ScopeKindPath, FromPath: "/left", ToPath: "/right"}, "plan:psql"); got != "diff --from-path /left --to-path /right plan:psql" {
			t.Fatalf("formatDiffCommandLine path = %q", got)
		}
		got := formatDiffCommandLine(Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefMode: "blob", RefKeepWorktree: true}, "prepare:lb")
		if !strings.Contains(got, "diff --from-ref main --to-ref HEAD --ref-mode blob --ref-keep-worktree prepare:lb") {
			t.Fatalf("formatDiffCommandLine ref = %q", got)
		}

		if got := scopeToJSONScope(Scope{Kind: ScopeKindPath, FromPath: "/left", ToPath: "/right"}); got.Mode != string(ScopeKindPath) || got.FromPath != "/left" || got.ToPath != "/right" {
			t.Fatalf("scopeToJSONScope path = %+v", got)
		}
		if got := scopeToJSONScope(Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefMode: "worktree", RefKeepWorktree: true}); got.Mode != string(ScopeKindRef) || got.FromRef != "main" || got.ToRef != "HEAD" || !got.RefKeepWorktree {
			t.Fatalf("scopeToJSONScope ref = %+v", got)
		}
	})

	t.Run("file snippet and render branches", func(t *testing.T) {
		if got := fileSnippet("", "anything.sql"); got != "" {
			t.Fatalf("fileSnippet empty root = %q", got)
		}

		root := t.TempDir()
		if got := fileSnippet(root, "missing.sql"); !strings.HasPrefix(got, "<error:") {
			t.Fatalf("fileSnippet missing = %q", got)
		}

		shortPath := filepath.Join(root, "short.sql")
		if err := os.WriteFile(shortPath, []byte("short\n"), 0o600); err != nil {
			t.Fatalf("write short snippet: %v", err)
		}
		if got := fileSnippet(root, "short.sql"); got != "short\n" {
			t.Fatalf("fileSnippet short = %q", got)
		}

		largePath := filepath.Join(root, "large.sql")
		if err := os.WriteFile(largePath, bytes.Repeat([]byte("x"), maxSnippetBytes+32), 0o600); err != nil {
			t.Fatalf("write large snippet: %v", err)
		}
		if got := fileSnippet(root, "large.sql"); !strings.Contains(got, "... (truncated)") {
			t.Fatalf("fileSnippet large = %q", got)
		}

		fromRoot := filepath.Join(root, "from")
		toRoot := filepath.Join(root, "to")
		if err := os.MkdirAll(fromRoot, 0o755); err != nil {
			t.Fatalf("mkdir fromRoot: %v", err)
		}
		if err := os.MkdirAll(toRoot, 0o755); err != nil {
			t.Fatalf("mkdir toRoot: %v", err)
		}
		for _, item := range []struct {
			root string
			name string
			body string
		}{
			{fromRoot, "removed.sql", "removed\n"},
			{toRoot, "added.sql", "added\n"},
			{toRoot, "modified.sql", "modified\n"},
		} {
			if err := os.WriteFile(filepath.Join(item.root, item.name), []byte(item.body), 0o600); err != nil {
				t.Fatalf("write %s: %v", item.name, err)
			}
		}

		result := DiffResult{
			Added:    []FileEntry{{Path: "added.sql", Hash: "ha"}, {Path: "added-2.sql", Hash: "hb"}},
			Modified: []FileEntry{{Path: "modified.sql", Hash: "hm"}, {Path: "modified-2.sql", Hash: "hn"}},
			Removed:  []FileEntry{{Path: "removed.sql", Hash: "hr"}, {Path: "removed-2.sql", Hash: "hs"}},
		}
		var human bytes.Buffer
		RenderHuman(&human, result, Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefKeepWorktree: true}, "plan:psql", Options{Limit: 1}, Context{Root: fromRoot}, Context{Root: toRoot})
		humanOut := human.String()
		if !strings.Contains(humanOut, "diff --from-ref main --to-ref HEAD --ref-keep-worktree plan:psql") {
			t.Fatalf("RenderHuman command line = %q", humanOut)
		}
		if strings.Count(humanOut, "... (1 more)") != 3 {
			t.Fatalf("expected truncation markers in human output:\n%s", humanOut)
		}

		var jsonBuf bytes.Buffer
		if err := RenderJSON(&jsonBuf, result, Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefMode: "worktree", RefKeepWorktree: true}, "prepare:lb", Options{Limit: 1, IncludeContent: true}, Context{Root: fromRoot}, Context{Root: toRoot}); err != nil {
			t.Fatalf("RenderJSON: %v", err)
		}
		var rendered JSONOutput
		if err := json.Unmarshal(jsonBuf.Bytes(), &rendered); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		if rendered.Scope.Mode != string(ScopeKindRef) || rendered.Scope.FromRef != "main" || rendered.Scope.ToRef != "HEAD" || !rendered.Scope.RefKeepWorktree {
			t.Fatalf("unexpected JSON scope: %+v", rendered.Scope)
		}
		if rendered.Command != "prepare:lb" {
			t.Fatalf("unexpected JSON command: %q", rendered.Command)
		}
		if len(rendered.Added) != 1 || len(rendered.Modified) != 1 || len(rendered.Removed) != 1 {
			t.Fatalf("expected truncated JSON entries: %+v", rendered)
		}
		if rendered.Added[0].Content == "" || rendered.Modified[0].Content == "" || rendered.Removed[0].Content == "" {
			t.Fatalf("expected JSON content snippets: %+v", rendered)
		}
	})
}

func TestDiffResolveRefErrorBranches(t *testing.T) {
	t.Run("missing git on path", func(t *testing.T) {
		workspace := t.TempDir()
		t.Setenv("PATH", t.TempDir())
		_, _, _, err := ResolveScope(Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefMode: "worktree"}, workspace)
		if err == nil || !strings.Contains(err.Error(), "git not found in PATH") {
			t.Fatalf("expected git lookup error, got %v", err)
		}
	})

	t.Run("non git repo", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not in PATH")
		}
		workspace := t.TempDir()
		_, _, _, err := ResolveScope(Scope{Kind: ScopeKindRef, FromRef: "main", ToRef: "HEAD", RefMode: "worktree"}, workspace)
		if err == nil || !strings.Contains(err.Error(), "not a git repository") {
			t.Fatalf("expected git repository error, got %v", err)
		}
	})

	t.Run("git helper failures", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not in PATH")
		}
		repoRoot := t.TempDir()
		if _, err := gitTopLevel(repoRoot); err == nil {
			t.Fatalf("expected gitTopLevel to fail")
		}
		if err := gitWorktreeAddDetach(repoRoot, filepath.Join(repoRoot, "from"), "HEAD"); err == nil {
			t.Fatalf("expected gitWorktreeAddDetach to fail")
		}
		if err := gitWorktreeRemove(repoRoot, filepath.Join(repoRoot, "from")); err == nil {
			t.Fatalf("expected gitWorktreeRemove to fail")
		}
	})

	t.Run("keep worktree cleanup", func(t *testing.T) {
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
			t.Skipf("git init skipped: %v\n%s", err, out)
		}
		runGit("config", "user.email", "t@e.st")
		runGit("config", "user.name", "t")
		file := filepath.Join(repo, "a.sql")
		if err := os.WriteFile(file, []byte("v1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit("add", "a.sql")
		runGit("commit", "-m", "first")
		headRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		head := strings.TrimSpace(string(headRef))
		fromCtx, toCtx, cleanup, err := ResolveScope(Scope{
			Kind:            ScopeKindRef,
			FromRef:         head,
			ToRef:           head,
			RefMode:         "worktree",
			RefKeepWorktree: true,
		}, repo)
		if err != nil {
			t.Fatalf("ResolveScope keep worktree: %v", err)
		}
		if cleanup == nil {
			t.Fatal("expected cleanup for keep-worktree scope")
		}
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup keep worktree: %v", err)
		}
		if err := gitWorktreeRemove(repo, fromCtx.Root); err != nil {
			t.Fatalf("remove from worktree: %v", err)
		}
		if err := gitWorktreeRemove(repo, toCtx.Root); err != nil {
			t.Fatalf("remove to worktree: %v", err)
		}
		if _, _, _, err := ResolveScope(Scope{
			Kind:    ScopeKindRef,
			FromRef: "missing-ref",
			ToRef:   head,
			RefMode: "worktree",
		}, repo); err == nil || !strings.Contains(err.Error(), "from-ref") {
			t.Fatalf("expected from-ref worktree error, got %v", err)
		}
		if _, _, _, err := ResolveScope(Scope{
			Kind:    ScopeKindRef,
			FromRef: head,
			ToRef:   "missing-ref",
			RefMode: "worktree",
		}, repo); err == nil || !strings.Contains(err.Error(), "to-ref") {
			t.Fatalf("expected to-ref worktree error, got %v", err)
		}
	})

	t.Run("cleanup reports worktree removal errors", func(t *testing.T) {
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
			t.Skipf("git init skipped: %v\n%s", err, out)
		}
		runGit("config", "user.email", "t@e.st")
		runGit("config", "user.name", "t")
		file := filepath.Join(repo, "a.sql")
		if err := os.WriteFile(file, []byte("v1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit("add", "a.sql")
		runGit("commit", "-m", "first")
		headRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		head := strings.TrimSpace(string(headRef))
		fromCtx, toCtx, cleanup, err := ResolveScope(Scope{
			Kind:    ScopeKindRef,
			FromRef: head,
			ToRef:   head,
			RefMode: "worktree",
		}, repo)
		if err != nil {
			t.Fatalf("ResolveScope cleanup error: %v", err)
		}
		if cleanup == nil {
			t.Fatal("expected cleanup for ref scope")
		}
		if err := os.RemoveAll(repo); err != nil {
			t.Fatalf("remove repo root: %v", err)
		}
		if err := cleanup(); err == nil {
			t.Fatal("expected cleanup to report worktree removal errors")
		}
		_ = os.RemoveAll(fromCtx.Root)
		_ = os.RemoveAll(toCtx.Root)
	})
}
