package app

import (
	"io"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

func runPlan(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string) error {
	return runPlanKindWithPathMode(stdout, stderr, runOpts, cfg, workspaceRoot, cwd, args, output, "psql", true)
}

func runPlanKind(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string, kind string) error {
	return runPlanKindWithPathMode(stdout, stderr, runOpts, cfg, workspaceRoot, cwd, args, output, kind, true)
}

func runPlanKindWithPathMode(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string, kind string, relativizeLiquibasePaths bool) error {
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintPlanUsage(stdout)
		return nil
	}
	return runPlanKindParsedWithPathMode(stdout, stderr, runOpts, cfg, workspaceRoot, cwd, parsed, nil, output, kind, relativizeLiquibasePaths)
}

func runPlanKindParsedWithPathMode(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, parsed prepareArgs, ref *refctx.Context, output string, kind string, relativizeLiquibasePaths bool) error {
	return runPlanStageRequest(stdout, stderr, runOpts, cfg, stageRunRequest{
		mode:                    stageModePlan,
		class:                   "raw",
		kind:                    kind,
		parsed:                  parsed,
		workspaceRoot:           workspaceRoot,
		cwd:                     cwd,
		invocationCwd:           cwd,
		ref:                     ref,
		output:                  output,
		relativizeLiquibasePath: relativizeLiquibasePaths,
	})
}

func runPlanStageRequest(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, req stageRunRequest) error {
	runtime, err := buildStageRuntime(stderr, runOpts, cfg, req)
	if err != nil {
		return err
	}
	return executePlanStageWithProvenance(stdout, runtime, req.output, req.parsed.ProvenancePath)
}
