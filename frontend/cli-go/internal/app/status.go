package app

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/sqlrs/cli/internal/cli"
)

type statusOptions struct {
	CacheDetails bool
}

func parseStatusFlags(args []string) (statusOptions, bool, error) {
	var opts statusOptions
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return opts, false, err
	}

	fs := flag.NewFlagSet("sqlrs status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cache := fs.Bool("cache", false, "show full cache diagnostics")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return opts, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return opts, true, nil
	}
	if fs.NArg() > 0 {
		return opts, false, fmt.Errorf("status does not accept arguments")
	}
	opts.CacheDetails = *cache
	return opts, false, nil
}

func runStatus(w io.Writer, runOpts cli.StatusOptions, workspaceRoot string, output string, args []string) error {
	opts, showHelp, err := parseStatusFlags(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintStatusUsage(w)
		return nil
	}

	runOpts.CacheDetails = opts.CacheDetails

	result, err := cli.RunStatus(context.Background(), runOpts)
	if err != nil {
		return err
	}

	result.Client = Version
	result.Workspace = workspaceRoot
	if output == "json" {
		if err := writeJSON(w, result); err != nil {
			return err
		}
	} else {
		cli.PrintStatus(w, result)
	}
	if !result.OK {
		return fmt.Errorf("service unhealthy")
	}
	return nil
}
