package app

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/config"
)

type cacheInvocation struct {
	stageName string
	stageArgs []string
}

func parseCacheArgs(args []string) (cacheInvocation, bool, error) {
	var invocation cacheInvocation
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return invocation, false, err
	}
	if len(args) == 0 {
		return invocation, false, ExitErrorf(2, "cache explain requires a wrapped prepare stage")
	}
	if args[0] == "--help" || args[0] == "-h" {
		return invocation, true, nil
	}
	if args[0] != "explain" {
		return invocation, false, ExitErrorf(2, "cache only supports explain")
	}
	if len(args) == 1 {
		return invocation, false, ExitErrorf(2, "cache explain requires a wrapped prepare stage")
	}
	if args[1] == "--help" || args[1] == "-h" {
		return invocation, true, nil
	}
	invocation.stageName = strings.TrimSpace(args[1])
	invocation.stageArgs = append([]string{}, args[2:]...)
	if invocation.stageName != "prepare" && !strings.HasPrefix(invocation.stageName, "prepare:") {
		return invocation, false, ExitErrorf(2, "cache explain only supports prepare stages")
	}
	return invocation, false, nil
}

func runCache(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string) error {
	invocation, showHelp, err := parseCacheArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintCacheUsage(stdout)
		return nil
	}

	switch invocation.stageName {
	case "prepare":
		aliasInvocation, showHelp, err := parsePrepareAliasArgs(invocation.stageArgs)
		if err != nil {
			return err
		}
		if showHelp {
			cli.PrintCacheUsage(stdout)
			return nil
		}
		if aliasInvocation.WatchSpecified {
			return ExitErrorf(2, "cache explain does not support --watch/--no-watch")
		}
		if strings.TrimSpace(aliasInvocation.ProvenancePath) != "" {
			return ExitErrorf(2, "cache explain does not support --provenance-path")
		}
		alias, aliasPath, ref, err := resolvePrepareAliasWithOptionalRef(workspaceRoot, cwd, aliasInvocation.Ref, aliasInvocation.GitRef, aliasInvocation.RefMode, aliasInvocation.RefKeepWorktree)
		if err != nil {
			return err
		}
		alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)
		return runCacheStage(stdout, stderr, runOpts, cfg, stageRunRequest{
			mode:                    stageModePrepare,
			class:                   "alias",
			kind:                    alias.Kind,
			parsed:                  prepareArgs{Image: alias.Image, PsqlArgs: alias.Args},
			workspaceRoot:           workspaceRoot,
			cwd:                     cacheAliasCWD(alias.Kind, aliasPath, cwd),
			invocationCwd:           cwd,
			aliasPath:               aliasPath,
			ref:                     ref,
			output:                  output,
			relativizeLiquibasePath: alias.Kind != "lb",
		})
	default:
		parsed, showHelp, err := parsePrepareArgs(invocation.stageArgs)
		if err != nil {
			return err
		}
		if showHelp {
			cli.PrintCacheUsage(stdout)
			return nil
		}
		if parsed.WatchSpecified {
			return ExitErrorf(2, "cache explain does not support --watch/--no-watch")
		}
		if strings.TrimSpace(parsed.ProvenancePath) != "" {
			return ExitErrorf(2, "cache explain does not support --provenance-path")
		}
		kind := strings.TrimSpace(strings.TrimPrefix(invocation.stageName, "prepare:"))
		if kind == "" {
			kind = "psql"
		}
		return runCacheStage(stdout, stderr, runOpts, cfg, stageRunRequest{
			mode:                    stageModePrepare,
			class:                   "raw",
			kind:                    kind,
			parsed:                  parsed,
			workspaceRoot:           workspaceRoot,
			cwd:                     cwd,
			invocationCwd:           cwd,
			output:                  output,
			relativizeLiquibasePath: kind != "lb",
		})
	}
}

func runCacheStage(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, req stageRunRequest) error {
	runtime, err := buildStageRuntime(stderr, runOpts, cfg, req)
	if err != nil {
		return err
	}
	if err := ensurePrepareTrace(&runtime); err != nil {
		return finishPrepareCleanup(err, runtime.cleanup)
	}
	explain, err := explainPrepareCacheFn(context.Background(), runtime.opts)
	if err != nil {
		return finishPrepareCleanup(err, runtime.cleanup)
	}
	return finishPrepareCleanup(cli.PrintCacheExplain(stdout, cacheExplainResult(runtime.trace, explain), req.output), runtime.cleanup)
}

func cacheAliasCWD(kind string, aliasPath string, cwd string) string {
	if strings.TrimSpace(strings.ToLower(kind)) == "lb" {
		return filepath.Dir(aliasPath)
	}
	return cwd
}
