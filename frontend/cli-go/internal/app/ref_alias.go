package app

import (
	aliaspkg "github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/refctx"
)

func resolvePrepareAliasWithOptionalRef(workspaceRoot string, cwd string, aliasRef string, gitRef string, refMode string, keepWorktree bool) (aliaspkg.Definition, string, *refctx.Context, error) {
	if gitRef == "" {
		target, err := aliaspkg.ResolveTarget(aliaspkg.ResolveOptions{
			WorkspaceRoot: workspaceRoot,
			CWD:           cwd,
			Ref:           aliasRef,
			Class:         aliaspkg.ClassPrepare,
		})
		if err != nil {
			return aliaspkg.Definition{}, "", nil, wrapAliasResolveError(aliaspkg.ClassPrepare, err)
		}
		alias, err := aliaspkg.LoadTarget(target)
		if err != nil {
			return aliaspkg.Definition{}, "", nil, wrapAliasLoadError(err)
		}
		return alias, target.Path, nil, nil
	}

	ctx, err := refctx.Resolve(workspaceRoot, cwd, gitRef, refMode, keepWorktree)
	if err != nil {
		return aliaspkg.Definition{}, "", nil, err
	}
	target, err := aliaspkg.ResolveTargetWithFS(aliaspkg.ResolveOptions{
		WorkspaceRoot: ctx.WorkspaceRoot,
		CWD:           ctx.BaseDir,
		Ref:           aliasRef,
		Class:         aliaspkg.ClassPrepare,
	}, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, wrapAliasResolveError(aliaspkg.ClassPrepare, err)
	}
	alias, err := aliaspkg.LoadTargetWithFS(target, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, wrapAliasLoadError(err)
	}
	return alias, target.Path, &ctx, nil
}
