package app

import (
	"io"
	"runtime"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/discover"
)

var analyzeDiscoverFn = discover.Analyze

func parseDiscoverArgs(args []string) (bool, []string, error) {
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return false, nil, err
	}
	selected := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return true, nil, nil
		case "--aliases":
			selected = append(selected, discover.AnalyzerAliases)
		case "--gitignore":
			selected = append(selected, discover.AnalyzerGitignore)
		case "--vscode":
			selected = append(selected, discover.AnalyzerVSCode)
		case "--prepare-shaping":
			selected = append(selected, discover.AnalyzerPrepareShaping)
		default:
			if strings.HasPrefix(arg, "-") {
				return false, nil, ExitErrorf(2, "unknown discover option: %s", arg)
			}
			return false, nil, ExitErrorf(2, "discover does not accept arguments")
		}
	}
	return false, selected, nil
}

func runDiscover(stdout io.Writer, stderr io.Writer, cmdCtx commandContext, args []string, output string) error {
	showHelp, selected, err := parseDiscoverArgs(args)
	if err != nil {
		return err
	}
	selected, err = discover.NormalizeSelectedAnalyzers(selected)
	if err != nil {
		return ExitErrorf(2, err.Error())
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

	shellFamily := discover.ShellFamilyPOSIX
	if runtime.GOOS == "windows" {
		shellFamily = discover.ShellFamilyPowerShell
	}

	report, err := analyzeDiscoverFn(discover.Options{
		WorkspaceRoot:     cmdCtx.workspaceRoot,
		CWD:               cmdCtx.cwd,
		SelectedAnalyzers: selected,
		ShellFamily:       shellFamily,
		Progress:          progress,
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
