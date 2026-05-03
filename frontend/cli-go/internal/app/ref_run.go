package app

import (
	"strings"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/refctx"
)

// resolveRunAliasWithOptionalRef keeps alias loading and git-ref projection on
// the same filesystem, matching docs/architecture/run-ref-component-structure.md.
func resolveRunAliasWithOptionalRef(workspaceRoot string, cwd string, aliasRef string, gitRef string, refMode string, keepWorktree bool) (aliaspkg.Definition, string, *refctx.Context, error) {
	if gitRef == "" {
		alias, aliasPath, err := resolveRunAliasDefinition(workspaceRoot, cwd, aliasRef)
		if err != nil {
			return aliaspkg.Definition{}, "", nil, err
		}
		return alias, aliasPath, nil, nil
	}

	ctx, err := refctx.Resolve(workspaceRoot, cwd, gitRef, refMode, keepWorktree)
	if err != nil {
		return aliaspkg.Definition{}, "", nil, err
	}
	target, err := aliaspkg.ResolveTargetWithFS(aliaspkg.ResolveOptions{
		WorkspaceRoot: ctx.WorkspaceRoot,
		CWD:           ctx.BaseDir,
		Ref:           aliasRef,
		Class:         aliaspkg.ClassRun,
	}, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, wrapAliasResolveError(aliaspkg.ClassRun, err)
	}
	alias, err := aliaspkg.LoadTargetWithFS(target, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, wrapAliasLoadError(err)
	}
	return alias, target.Path, &ctx, nil
}

// resolveRunBindingContext centralizes the standalone run-stage ref ownership
// boundary so detached worktrees are cleaned exactly once.
func resolveRunBindingContext(workspaceRoot string, cwd string, parsed runArgs, existing *refctx.Context) (*refctx.Context, func() error, error) {
	if existing != nil {
		return existing, existing.Cleanup, nil
	}
	if strings.TrimSpace(parsed.Ref) == "" {
		return nil, nil, nil
	}
	ctx, err := refctx.Resolve(workspaceRoot, cwd, parsed.Ref, parsed.RefMode, parsed.RefKeepWorktree)
	if err != nil {
		return nil, nil, err
	}
	return &ctx, ctx.Cleanup, nil
}

// projectedRunInvocationCWD preserves run alias command-source semantics under
// `--ref`: plain `\i` / `\include` still resolve from the caller's projected
// cwd, while explicit file args are rebased separately from the alias file.
func projectedRunInvocationCWD(invocationCWD string, ctx *refctx.Context) string {
	if ctx == nil || strings.TrimSpace(ctx.BaseDir) == "" {
		return invocationCWD
	}
	return ctx.BaseDir
}
