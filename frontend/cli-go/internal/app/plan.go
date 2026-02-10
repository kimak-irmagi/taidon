package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

func runPlan(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string) error {
	return runPlanKind(stdout, stderr, runOpts, cfg, workspaceRoot, cwd, args, output, "psql")
}

func runPlanKind(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, workspaceRoot string, cwd string, args []string, output string, kind string) error {
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintPlanUsage(stdout)
		return nil
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

	switch kind {
	case "psql":
		psqlArgs, stdin, err := normalizePsqlArgs(parsed.PsqlArgs, workspaceRoot, cwd, os.Stdin, buildPathConverter(runOpts))
		if err != nil {
			return err
		}
		runOpts.ImageID = imageID
		runOpts.PsqlArgs = psqlArgs
		runOpts.Stdin = stdin
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
		converter := buildPathConverter(runOpts)
		if shouldUseLiquibaseWindowsMode(liquibaseExec, liquibaseExecMode) {
			converter = nil
		}
		liquibaseArgs, err := normalizeLiquibaseArgs(parsed.PsqlArgs, workspaceRoot, cwd, converter)
		if err != nil {
			return err
		}
		if shouldUseLiquibaseWindowsMode(liquibaseExec, liquibaseExecMode) {
			liquibaseArgs = relativizeLiquibaseArgs(liquibaseArgs, workspaceRoot, cwd)
		}
		liquibaseEnv := resolveLiquibaseEnv()
		workDir, err := normalizeWorkDir(cwd, converter)
		if err != nil {
			return err
		}
		runOpts.ImageID = imageID
		runOpts.LiquibaseArgs = liquibaseArgs
		runOpts.LiquibaseExec = liquibaseExec
		runOpts.LiquibaseExecMode = liquibaseExecMode
		runOpts.LiquibaseEnv = liquibaseEnv
		runOpts.WorkDir = workDir
		runOpts.PrepareKind = "lb"
		runOpts.PlanOnly = true
	default:
		return ExitErrorf(2, "unsupported plan kind: %s", kind)
	}

	result, err := cli.RunPlan(context.Background(), runOpts)
	if err != nil {
		return err
	}
	if output == "json" {
		return writeJSON(stdout, result)
	}
	return cli.PrintPlan(stdout, result)
}
