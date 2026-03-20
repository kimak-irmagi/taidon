package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/cli"
)

// aliasCommand wires the CLI-facing contract documented in
// docs/architecture/alias-inspection-component-structure.md.
type aliasCommand struct {
	Action  string
	Prepare bool
	Run     bool
	From    string
	Depth   string
	Ref     string
}

func parseAliasArgs(args []string) (aliasCommand, bool, error) {
	var cmd aliasCommand
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return cmd, false, err
	}
	if len(args) == 0 {
		return cmd, false, ExitErrorf(2, "missing alias subcommand")
	}
	switch args[0] {
	case "--help", "-h":
		return cmd, true, nil
	case "ls", "check":
		cmd.Action = args[0]
	default:
		return cmd, false, ExitErrorf(2, "unknown alias subcommand: %s", args[0])
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return cmd, true, nil
		case arg == "--prepare":
			cmd.Prepare = true
		case arg == "--run":
			cmd.Run = true
		case arg == "--from":
			if i+1 >= len(args) {
				return cmd, false, ExitErrorf(2, "Missing value for --from")
			}
			cmd.From = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--from="):
			cmd.From = strings.TrimSpace(strings.TrimPrefix(arg, "--from="))
		case arg == "--depth":
			if i+1 >= len(args) {
				return cmd, false, ExitErrorf(2, "Missing value for --depth")
			}
			cmd.Depth = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--depth="):
			cmd.Depth = strings.TrimSpace(strings.TrimPrefix(arg, "--depth="))
		default:
			if strings.HasPrefix(arg, "-") {
				return cmd, false, ExitErrorf(2, "unknown alias option: %s", arg)
			}
			if cmd.Ref != "" {
				return cmd, false, ExitErrorf(2, "alias check accepts at most one alias ref")
			}
			cmd.Ref = strings.TrimSpace(arg)
		}
	}

	if cmd.Action == "ls" && cmd.Ref != "" {
		return cmd, false, ExitErrorf(2, "alias ls does not accept an alias ref")
	}
	if cmd.Action == "check" && cmd.Ref != "" && (strings.TrimSpace(cmd.From) != "" || strings.TrimSpace(cmd.Depth) != "") {
		return cmd, false, ExitErrorf(2, "alias check <ref> does not accept --from or --depth")
	}
	return cmd, false, nil
}

func runAliasCommand(stdout io.Writer, cmdCtx commandContext, args []string) error {
	cmd, showHelp, err := parseAliasArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintAliasUsage(stdout)
		return nil
	}

	scanOpts := alias.ScanOptions{
		WorkspaceRoot: cmdCtx.workspaceRoot,
		CWD:           cmdCtx.cwd,
		From:          cmd.From,
		Depth:         cmd.Depth,
		Classes:       selectedAliasClasses(cmd),
	}

	switch cmd.Action {
	case "ls":
		entries, err := alias.Scan(scanOpts)
		if err != nil {
			return ExitErrorf(2, err.Error())
		}
		if cmdCtx.output == "json" {
			return writeJSON(stdout, entries)
		}
		cli.PrintAliasEntries(stdout, entries)
		return nil
	case "check":
		report, err := runAliasCheck(cmdCtx, scanOpts, cmd)
		if err != nil {
			return err
		}
		if cmdCtx.output == "json" {
			if err := writeJSON(stdout, report); err != nil {
				return err
			}
		} else {
			cli.PrintAliasCheck(stdout, report)
		}
		if report.InvalidCount > 0 {
			return ExitErrorf(1, "alias check reported invalid aliases")
		}
		return nil
	default:
		return fmt.Errorf("unknown alias subcommand: %s", cmd.Action)
	}
}

func runAliasCheck(cmdCtx commandContext, scanOpts alias.ScanOptions, cmd aliasCommand) (alias.CheckReport, error) {
	if cmd.Ref == "" {
		report, err := alias.CheckScan(scanOpts)
		if err != nil {
			return alias.CheckReport{}, ExitErrorf(2, err.Error())
		}
		return report, nil
	}

	target, err := alias.ResolveTarget(alias.ResolveOptions{
		WorkspaceRoot: cmdCtx.workspaceRoot,
		CWD:           cmdCtx.cwd,
		Ref:           cmd.Ref,
		Class:         selectedSingleAliasClass(cmd),
	})
	if err != nil {
		return alias.CheckReport{}, ExitErrorf(2, err.Error())
	}
	result, err := alias.CheckTarget(target, cmdCtx.workspaceRoot)
	if err != nil {
		return alias.CheckReport{}, err
	}
	report := alias.CheckReport{
		Checked: 1,
		Results: []alias.CheckResult{result},
	}
	if result.Valid {
		report.ValidCount = 1
	} else {
		report.InvalidCount = 1
	}
	return report, nil
}

func selectedAliasClasses(cmd aliasCommand) []alias.Class {
	classes := make([]alias.Class, 0, 2)
	if cmd.Prepare {
		classes = append(classes, alias.ClassPrepare)
	}
	if cmd.Run {
		classes = append(classes, alias.ClassRun)
	}
	return classes
}

func selectedSingleAliasClass(cmd aliasCommand) alias.Class {
	switch {
	case cmd.Prepare && !cmd.Run:
		return alias.ClassPrepare
	case cmd.Run && !cmd.Prepare:
		return alias.ClassRun
	default:
		return ""
	}
}
