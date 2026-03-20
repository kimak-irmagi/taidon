package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/diff"
)

func runDiff(stdout, stderr io.Writer, cwd string, args []string, outputFormat string, verbose bool) error {
	parsed, err := diff.ParseDiffScope(args)
	if err != nil {
		return ExitErrorf(2, "%s", err.Error())
	}
	if verbose && stderr != nil {
		switch parsed.Scope.Kind {
		case diff.ScopeKindRef:
			fmt.Fprintf(stderr, "diff scope: --from-ref %s --to-ref %s (ref-mode=%s)\n",
				parsed.Scope.FromRef, parsed.Scope.ToRef, parsed.Scope.RefMode)
		default:
			fmt.Fprintf(stderr, "diff scope: --from-path %s --to-path %s\n", parsed.Scope.FromPath, parsed.Scope.ToPath)
		}
		fmt.Fprintf(stderr, "wrapped command: %s %s\n", parsed.WrappedName, strings.Join(parsed.WrappedArgs, " "))
	}
	format := strings.TrimSpace(strings.ToLower(outputFormat))
	if format == "" {
		format = "human"
	}
	if format != "human" && format != "json" {
		return ExitErrorf(2, "invalid output format for diff: %s (use human or json)", outputFormat)
	}
	if err := cli.RunDiff(stdout, parsed, cwd, format); err != nil {
		return ExitErrorf(2, "%s", err.Error())
	}
	return nil
}
