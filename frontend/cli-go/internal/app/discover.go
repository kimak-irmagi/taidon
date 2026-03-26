package app

import (
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/discover"
)

var analyzeAliasesFn = discover.AnalyzeAliases

func parseDiscoverArgs(args []string) (bool, error) {
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return false, err
	}
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return true, nil
		case "--aliases":
			continue
		default:
			if strings.HasPrefix(arg, "-") {
				return false, ExitErrorf(2, "unknown discover option: %s", arg)
			}
			return false, ExitErrorf(2, "discover does not accept arguments")
		}
	}
	return false, nil
}

func runDiscover(stdout io.Writer, stderr io.Writer, cmdCtx commandContext, args []string, output string) error {
	showHelp, err := parseDiscoverArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintDiscoverUsage(stdout)
		return nil
	}

	stopSpinner := func() {}
	if !cmdCtx.verbose {
		stopSpinner = startSpinner("discover: scanning workspace", false)
	}

	var progress discover.Progress
	if cmdCtx.verbose {
		progress = newDiscoverProgressWriter(stderr)
	}

	report, err := analyzeAliasesFn(discover.Options{
		WorkspaceRoot: cmdCtx.workspaceRoot,
		CWD:           cmdCtx.cwd,
		Progress:      progress,
	})
	stopSpinner()
	if err != nil {
		return err
	}

	if output == "json" {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
		return nil
	}
	cli.PrintDiscover(stdout, report)
	return nil
}
