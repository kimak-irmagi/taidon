package diff

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
)

// ResolveScope turns a Scope into two Context values. Ref mode with RefMode
// "blob" (default) reads objects via git show/ls-tree without checkout; cleanup
// is nil. RefMode "worktree" creates temporary git worktrees and returns a
// non-nil cleanup unless RefKeepWorktree is set. Path mode cleanup is nil.
func ResolveScope(s Scope, cwd string) (fromCtx, toCtx Context, cleanup func() error, err error) {
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	switch s.Kind {
	case ScopeKindPath:
		fromCtx, toCtx, err = resolvePathScopeStrings(s.FromPath, s.ToPath, cwd)
		return fromCtx, toCtx, nil, err
	case ScopeKindRef:
		if strings.TrimSpace(s.RefMode) == "worktree" {
			return resolveRefWorktrees(s, cwd)
		}
		return resolveRefGitObjects(s, cwd)
	default:
		return Context{}, Context{}, nil, fmt.Errorf("diff: unknown scope kind %q", s.Kind)
	}
}

func resolvePathScopeStrings(fromPath, toPath, cwd string) (fromCtx, toCtx Context, err error) {
	fromAbs, err := absPathInCwd(fromPath, cwd)
	if err != nil {
		return Context{}, Context{}, fmt.Errorf("from-path: %w", err)
	}
	toAbs, err := absPathInCwd(toPath, cwd)
	if err != nil {
		return Context{}, Context{}, fmt.Errorf("to-path: %w", err)
	}
	return Context{Root: fromAbs, BaseDir: fromAbs}, Context{Root: toAbs, BaseDir: toAbs}, nil
}

func absPathInCwd(p, cwd string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	if strings.TrimSpace(cwd) == "" {
		return filepath.Abs(filepath.Clean(p))
	}
	return filepath.Clean(filepath.Join(cwd, p)), nil
}

func resolveRefWorktrees(s Scope, cwd string) (fromCtx, toCtx Context, cleanup func() error, err error) {
	reason := refWorktreeUnavailable()
	if reason != "" {
		return Context{}, Context{}, nil, fmt.Errorf("git worktree: %s", reason)
	}
	repoRoot, err := gitTopLevel(cwd)
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("diff ref mode: %w", err)
	}
	relCwd, err := cwdWithinRepo(repoRoot, cwd)
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("diff ref mode: resolve cwd: %w", err)
	}
	fromDir, err := os.MkdirTemp("", "sqlrs-diff-from-*")
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("mkdir from worktree: %w", err)
	}
	toDir, err := os.MkdirTemp("", "sqlrs-diff-to-*")
	if err != nil {
		_ = os.RemoveAll(fromDir)
		return Context{}, Context{}, nil, fmt.Errorf("mkdir to worktree: %w", err)
	}
	cleanupBoth := func() error {
		if s.RefKeepWorktree {
			return nil
		}
		var errs []string
		if e := gitWorktreeRemove(repoRoot, fromDir); e != nil {
			errs = append(errs, fmt.Sprintf("remove from worktree: %v", e))
			_ = os.RemoveAll(fromDir)
		}
		if e := gitWorktreeRemove(repoRoot, toDir); e != nil {
			errs = append(errs, fmt.Sprintf("remove to worktree: %v", e))
			_ = os.RemoveAll(toDir)
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return nil
	}
	if err := gitWorktreeAddDetach(repoRoot, fromDir, s.FromRef); err != nil {
		_ = cleanupBoth()
		return Context{}, Context{}, nil, fmt.Errorf("from-ref %q: %w", s.FromRef, err)
	}
	if err := gitWorktreeAddDetach(repoRoot, toDir, s.ToRef); err != nil {
		_ = cleanupBoth()
		return Context{}, Context{}, nil, fmt.Errorf("to-ref %q: %w", s.ToRef, err)
	}
	fromRoot := inputset.CanonicalizeBoundaryPath(fromDir)
	toRoot := inputset.CanonicalizeBoundaryPath(toDir)
	fromBase := inputset.CanonicalizeBoundaryPath(filepath.Join(fromDir, relCwd))
	toBase := inputset.CanonicalizeBoundaryPath(filepath.Join(toDir, relCwd))
	return Context{
			Root:    fromRoot,
			BaseDir: fromBase,
		}, Context{
			Root:    toRoot,
			BaseDir: toBase,
		}, cleanupBoth, nil
}

// resolveRefGitObjects builds contexts that read file contents with git show /
// ls-tree at each ref (no worktree checkout). cleanup is always nil.
func resolveRefGitObjects(s Scope, cwd string) (fromCtx, toCtx Context, cleanup func() error, err error) {
	if refWorktreeUnavailable() != "" {
		return Context{}, Context{}, nil, fmt.Errorf("git: %s", refWorktreeUnavailable())
	}
	repoRoot, err := gitTopLevel(cwd)
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("diff ref mode: %w", err)
	}
	relCwd, err := cwdWithinRepo(repoRoot, cwd)
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("diff ref mode: resolve cwd: %w", err)
	}
	rootCanon := inputset.CanonicalizeBoundaryPath(filepath.Clean(repoRoot))
	baseDir := inputset.CanonicalizeBoundaryPath(filepath.Join(rootCanon, relCwd))
	return Context{
			Root:    rootCanon,
			BaseDir: baseDir,
			GitRef:  s.FromRef,
		}, Context{
			Root:    rootCanon,
			BaseDir: baseDir,
			GitRef:  s.ToRef,
		}, nil, nil
}

func cwdWithinRepo(repoRoot, cwd string) (string, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	absRepo = inputset.CanonicalizeBoundaryPath(absRepo)

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	absCwd = inputset.CanonicalizeBoundaryPath(absCwd)

	rel, err := filepath.Rel(absRepo, absCwd)
	if err != nil {
		return "", err
	}
	return filepath.Clean(rel), nil
}

func refWorktreeUnavailable() string {
	_, err := exec.LookPath("git")
	if err != nil {
		return "git not found in PATH"
	}
	return ""
}

func gitTopLevel(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("not a git repository (or git failed): %w", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

func gitWorktreeAddDetach(repoRoot, path, ref string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "--detach", path, ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitWorktreeRemove(repoRoot, path string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
