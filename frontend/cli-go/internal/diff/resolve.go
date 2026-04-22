package diff

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/refctx"
)

// ResolveScope turns a Scope into two Context values. Ref mode "worktree"
// (default) reuses the shared refctx worktree resolver; ref mode "blob"
// reuses the shared git-object-backed resolver.
func ResolveScope(s Scope, cwd string) (fromCtx, toCtx Context, cleanup func() error, err error) {
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	switch s.Kind {
	case ScopeKindPath:
		fromCtx, toCtx, err = resolvePathScopeStrings(s.FromPath, s.ToPath, cwd)
		return fromCtx, toCtx, nil, err
	case ScopeKindRef:
		return resolveRefScope(s, cwd)
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

func resolveRefScope(s Scope, cwd string) (fromCtx, toCtx Context, cleanup func() error, err error) {
	mode := strings.TrimSpace(strings.ToLower(s.RefMode))
	if mode == "" {
		mode = "worktree"
	}
	if mode != "worktree" && mode != "blob" {
		return Context{}, Context{}, nil, fmt.Errorf("diff: unknown ref mode %q", s.RefMode)
	}

	fromRefCtx, err := refctx.Resolve("", cwd, s.FromRef, mode, s.RefKeepWorktree)
	if err != nil {
		return Context{}, Context{}, nil, fmt.Errorf("from-ref %q: %w", s.FromRef, err)
	}

	toRefCtx, err := refctx.Resolve("", cwd, s.ToRef, mode, s.RefKeepWorktree)
	if err != nil {
		_ = fromRefCtx.Cleanup()
		return Context{}, Context{}, nil, fmt.Errorf("to-ref %q: %w", s.ToRef, err)
	}

	fromCtx = diffContextFromRef(fromRefCtx)
	toCtx = diffContextFromRef(toRefCtx)
	if mode == "blob" {
		return fromCtx, toCtx, nil, nil
	}
	return fromCtx, toCtx, joinRefContextCleanup(fromRefCtx, toRefCtx), nil
}

func diffContextFromRef(ctx refctx.Context) Context {
	diffCtx := Context{
		Root:    ctx.RepoRoot,
		BaseDir: ctx.BaseDir,
	}
	if ctx.RefMode == "blob" {
		diffCtx.GitRef = ctx.GitRef
	}
	return diffCtx
}

func joinRefContextCleanup(fromCtx, toCtx refctx.Context) func() error {
	return func() error {
		var errs []string
		if err := fromCtx.Cleanup(); err != nil {
			errs = append(errs, fmt.Sprintf("remove from worktree: %v", err))
		}
		if err := toCtx.Cleanup(); err != nil {
			errs = append(errs, fmt.Sprintf("remove to worktree: %v", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return nil
	}
}
