package app

import (
	aliaspkg "github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/refctx"
)

func resolvePrepareAliasWithOptionalRef(workspaceRoot string, cwd string, aliasRef string, gitRef string, refMode string, keepWorktree bool) (aliaspkg.Definition, string, *refctx.Context, error) {
	if gitRef == "" {
		aliasPath, err := resolvePrepareAliasPath(workspaceRoot, cwd, aliasRef)
		if err != nil {
			return aliaspkg.Definition{}, "", nil, err
		}
		alias, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassPrepare, Path: aliasPath})
		if err != nil {
			return aliaspkg.Definition{}, "", nil, err
		}
		return alias, aliasPath, nil, nil
	}

	ctx, err := refctx.Resolve(workspaceRoot, cwd, gitRef, refMode, keepWorktree)
	if err != nil {
		return aliaspkg.Definition{}, "", nil, err
	}
	aliasPath, err := resolvePrepareAliasPathWithFS(ctx.WorkspaceRoot, ctx.BaseDir, aliasRef, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, err
	}
	alias, err := aliaspkg.LoadTargetWithFS(aliaspkg.Target{Class: aliaspkg.ClassPrepare, Path: aliasPath}, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return aliaspkg.Definition{}, "", nil, err
	}
	return alias, aliasPath, &ctx, nil
}
