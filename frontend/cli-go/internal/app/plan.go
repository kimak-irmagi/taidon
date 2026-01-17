package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

func runPlan(stdout, stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, cwd string, args []string, output string) error {
	parsed, showHelp, err := parsePrepareArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintPlanUsage(stdout)
		return nil
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

	psqlArgs, stdin, err := normalizePsqlArgs(parsed.PsqlArgs, cwd, os.Stdin)
	if err != nil {
		return err
	}

	runOpts.ImageID = imageID
	runOpts.PsqlArgs = psqlArgs
	runOpts.Stdin = stdin
	runOpts.PrepareKind = "psql"
	runOpts.PlanOnly = true

	result, err := cli.RunPlan(context.Background(), runOpts)
	if err != nil {
		return err
	}
	if output == "json" {
		return writeJSON(stdout, result)
	}
	return cli.PrintPlan(stdout, result)
}
