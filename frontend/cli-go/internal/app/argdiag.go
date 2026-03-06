package app

import "github.com/sqlrs/cli/internal/cli"

func validateNoUnicodeDashFlags(args []string, exitCode int) error {
	if hint := cli.UnicodeDashFlagMessage(args); hint != "" {
		return ExitErrorf(exitCode, "Invalid arguments: %s", hint)
	}
	return nil
}
