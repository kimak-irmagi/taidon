package app

import "github.com/sqlrs/cli/internal/refctx"

func resolvePrepareAliasWithOptionalRef(workspaceRoot string, cwd string, aliasRef string, gitRef string, refMode string, keepWorktree bool) (prepareAlias, string, *refctx.Context, error) {
	if gitRef == "" {
		aliasPath, err := resolvePrepareAliasPath(workspaceRoot, cwd, aliasRef)
		if err != nil {
			return prepareAlias{}, "", nil, err
		}
		alias, err := loadPrepareAlias(aliasPath)
		if err != nil {
			return prepareAlias{}, "", nil, err
		}
		return alias, aliasPath, nil, nil
	}

	ctx, err := refctx.Resolve(workspaceRoot, cwd, gitRef, refMode, keepWorktree)
	if err != nil {
		return prepareAlias{}, "", nil, err
	}
	aliasPath, err := resolvePrepareAliasPathWithFS(ctx.WorkspaceRoot, ctx.BaseDir, aliasRef, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return prepareAlias{}, "", nil, err
	}
	alias, err := loadPrepareAliasWithFS(aliasPath, ctx.FileSystem)
	if err != nil {
		_ = ctx.Cleanup()
		return prepareAlias{}, "", nil, err
	}
	return alias, aliasPath, &ctx, nil
}
