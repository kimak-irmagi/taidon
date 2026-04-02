package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/cli"
)

func runAliasCreate(stdout io.Writer, cmdCtx commandContext, cmd aliasCommand) error {
	class, kind, err := parseAliasCreateWrappedCommand(cmd.Wrapped)
	if err != nil {
		return ExitErrorf(2, err.Error())
	}

	result, err := alias.Create(alias.CreateOptions{
		WorkspaceRoot: cmdCtx.workspaceRoot,
		CWD:           cmdCtx.cwd,
		Ref:           cmd.Ref,
		Class:         class,
		Kind:          kind,
		Args:          cmd.Args,
	})
	if err != nil {
		return ExitErrorf(2, err.Error())
	}

	if cmdCtx.output == "json" {
		if err := writeJSON(stdout, result); err != nil {
			return err
		}
		return nil
	}
	cli.PrintAliasCreate(stdout, result)
	return nil
}

func parseAliasCreateWrappedCommand(value string) (alias.Class, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", fmt.Errorf("missing wrapped command")
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("wrapped command must be prepare:<kind> or run:<kind>")
	}

	className := strings.ToLower(strings.TrimSpace(parts[0]))
	kind := strings.ToLower(strings.TrimSpace(parts[1]))
	if kind == "" {
		return "", "", fmt.Errorf("wrapped command kind is required")
	}

	switch className {
	case string(alias.ClassPrepare):
		return alias.ClassPrepare, kind, nil
	case string(alias.ClassRun):
		return alias.ClassRun, kind, nil
	default:
		return "", "", fmt.Errorf("wrapped command must be prepare:<kind> or run:<kind>")
	}
}
