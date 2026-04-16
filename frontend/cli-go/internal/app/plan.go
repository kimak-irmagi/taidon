package app

import (
	"context"
	"fmt"
	"io"
	"os"

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
	if parsed.WatchSpecified {
		return ExitErrorf(2, "plan does not support --watch/--no-watch")
	}

	if kind == "lb" && len(parsed.PsqlArgs) == 0 {
		return ExitErrorf(2, "liquibase command is required")
	}

	imageID, source, err := resolvePrepareImage(parsed.Image, cfg)
	if err != nil {
		return err
	}
	if imageID == "" {
		return ExitErrorf(2, "Missing base image id (set --image or dbms.image)")
	}
	if runOpts.Verbose {
		fmt.Fprint(stderr, formatImageSource(imageID, source))
	}

	var cleanup func() error
	switch kind {
	case "psql":
		bound, err := bindPreparePsqlInputsFn(runOpts, workspaceRoot, cwd, parsed, ref, os.Stdin)
		if err != nil {
			return err
		}
		cleanup = bound.cleanup
		runOpts.ImageID = imageID
		runOpts.PsqlArgs = bound.PsqlArgs
		runOpts.Stdin = bound.Stdin
		runOpts.PrepareKind = "psql"
		runOpts.PlanOnly = true
	case "lb":
		liquibaseExec, err := resolveLiquibaseExec(cfg)
		if err != nil {
			return err
		}
		liquibaseExecMode, err := resolveLiquibaseExecMode(cfg)
		if err != nil {
			return err
		}
		bound, err := bindPrepareLiquibaseInputsFn(runOpts, workspaceRoot, cwd, parsed, ref, liquibaseExec, liquibaseExecMode, relativizeLiquibasePaths)
		if err != nil {
			return err
		}
		cleanup = bound.cleanup
		runOpts.ImageID = imageID
		runOpts.LiquibaseArgs = bound.LiquibaseArgs
		runOpts.LiquibaseExec = liquibaseExec
		runOpts.LiquibaseExecMode = liquibaseExecMode
		runOpts.LiquibaseEnv = resolveLiquibaseEnv()
		runOpts.WorkDir = bound.WorkDir
		runOpts.PrepareKind = "lb"
		runOpts.PlanOnly = true
	default:
		return ExitErrorf(2, "unsupported plan kind: %s", kind)
	}

	result, err := runPlanFn(context.Background(), runOpts)
	if err != nil {
		return finishPrepareCleanup(err, cleanup)
	}
	if output == "json" {
		return finishPrepareCleanup(writeJSON(stdout, result), cleanup)
	}
	return finishPrepareCleanup(cli.PrintPlan(stdout, result), cleanup)
}
