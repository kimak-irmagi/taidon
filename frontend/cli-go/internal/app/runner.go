package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
)

// runner centralizes top-level CLI orchestration while keeping command-specific
// behavior in dedicated helpers, per docs/architecture/cli-maintainability-refactor.md.
type runner struct {
	deps runnerDeps
}

type runnerDeps struct {
	stdout io.Writer
	stderr io.Writer

	parseArgs             func([]string) (cli.GlobalOptions, []cli.Command, error)
	getwd                 func() (string, error)
	resolveCommandContext func(string, cli.GlobalOptions) (commandContext, error)

	runInit         func(io.Writer, string, string, []string, bool) error
	runDiff         func(io.Writer, io.Writer, string, []string, string, bool) error
	runAlias        func(io.Writer, commandContext, []string) error
	runDiscover     func(io.Writer, io.Writer, commandContext, []string, string) error
	runLs           func(io.Writer, cli.LsOptions, []string, string) error
	runRm           func(io.Writer, cli.RmOptions, []string, string) error
	runPrepare      func(io.Writer, io.Writer, cli.PrepareOptions, config.LoadedConfig, string, string, []string) error
	prepareResult   func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error)
	runPrepareLB    func(io.Writer, io.Writer, cli.PrepareOptions, config.LoadedConfig, string, string, []string) error
	prepareResultLB func(stdoutAndErr, cli.PrepareOptions, config.LoadedConfig, string, string, []string) (client.PrepareJobResult, bool, error)
	runPlan         func(io.Writer, io.Writer, cli.PrepareOptions, config.LoadedConfig, string, string, []string, string, string) error
	runRun          func(io.Writer, io.Writer, cli.RunOptions, string, []string, string, string) error
	runStatus       func(io.Writer, cli.StatusOptions, string, string, []string) error
	runWatch        func(io.Writer, cli.PrepareOptions, []string) error
	runConfig       func(io.Writer, cli.ConfigOptions, []string, string) error

	cleanupPreparedInstance func(context.Context, io.Writer, cli.RunOptions, string, bool)
}

func Run(args []string) error {
	return newDefaultRunner().run(args)
}

func newDefaultRunner() runner {
	return newRunner(runnerDeps{})
}

func newRunner(deps runnerDeps) runner {
	deps.withDefaults()
	return runner{deps: deps}
}

func (deps *runnerDeps) withDefaults() {
	if deps.stdout == nil {
		deps.stdout = os.Stdout
	}
	if deps.stderr == nil {
		deps.stderr = os.Stderr
	}
	if deps.parseArgs == nil {
		deps.parseArgs = cli.ParseArgs
	}
	if deps.getwd == nil {
		deps.getwd = os.Getwd
	}
	if deps.resolveCommandContext == nil {
		deps.resolveCommandContext = resolveCommandContext
	}
	if deps.runInit == nil {
		deps.runInit = runInit
	}
	if deps.runDiff == nil {
		deps.runDiff = runDiff
	}
	if deps.runAlias == nil {
		deps.runAlias = runAliasCommand
	}
	if deps.runDiscover == nil {
		deps.runDiscover = runDiscover
	}
	if deps.runLs == nil {
		deps.runLs = runLs
	}
	if deps.runRm == nil {
		deps.runRm = runRm
	}
	if deps.runPrepare == nil {
		deps.runPrepare = runPrepare
	}
	if deps.prepareResult == nil {
		deps.prepareResult = prepareResult
	}
	if deps.runPrepareLB == nil {
		deps.runPrepareLB = runPrepareLiquibase
	}
	if deps.prepareResultLB == nil {
		deps.prepareResultLB = prepareResultLiquibase
	}
	if deps.runPlan == nil {
		deps.runPlan = runPlanKind
	}
	if deps.runRun == nil {
		deps.runRun = runRun
	}
	if deps.runStatus == nil {
		deps.runStatus = runStatus
	}
	if deps.runWatch == nil {
		deps.runWatch = runWatch
	}
	if deps.runConfig == nil {
		deps.runConfig = runConfig
	}
	if deps.cleanupPreparedInstance == nil {
		deps.cleanupPreparedInstance = cleanupPreparedInstance
	}
}

func (r runner) run(args []string) error {
	opts, commands, err := r.deps.parseArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			cli.PrintUsage(r.deps.stdout)
			return nil
		}
		cli.PrintUsage(r.deps.stderr)
		return err
	}

	cwd, err := r.deps.getwd()
	if err != nil {
		return err
	}

	if len(commands) == 0 {
		return fmt.Errorf("missing command")
	}

	if commands[0].Name == "init" {
		if len(commands) > 1 {
			return fmt.Errorf("init cannot be combined with other commands")
		}
		return r.deps.runInit(r.deps.stdout, cwd, opts.Workspace, commands[0].Args, opts.Verbose)
	}

	if commands[0].Name == "diff" {
		if len(commands) > 1 {
			return fmt.Errorf("diff cannot be combined with other commands")
		}
		return r.deps.runDiff(r.deps.stdout, r.deps.stderr, cwd, commands[0].Args, opts.Output, opts.Verbose)
	}

	cmdCtx, err := r.deps.resolveCommandContext(cwd, opts)
	if err != nil {
		return err
	}

	var prepared *client.PrepareJobResult
	for idx, cmd := range commands {
		if idx == 0 && len(commands) > 1 && prepareStageUsesRef(cmd) {
			return fmt.Errorf("prepare --ref does not support composite run yet")
		}
		switch cmd.Name {
		case "alias":
			if len(commands) > 1 {
				return fmt.Errorf("alias cannot be combined with other commands")
			}
			return r.deps.runAlias(r.deps.stdout, cmdCtx, cmd.Args)
		case "discover":
			if len(commands) > 1 {
				return fmt.Errorf("discover cannot be combined with other commands")
			}
			return r.deps.runDiscover(r.deps.stdout, r.deps.stderr, cmdCtx, cmd.Args, cmdCtx.output)
		case "ls":
			if len(commands) > 1 {
				return fmt.Errorf("ls cannot be combined with other commands")
			}
			return r.deps.runLs(r.deps.stdout, cmdCtx.lsOptions(), cmd.Args, cmdCtx.output)
		case "rm":
			if len(commands) > 1 {
				return fmt.Errorf("rm cannot be combined with other commands")
			}
			return r.deps.runRm(r.deps.stdout, cmdCtx.rmOptions(), cmd.Args, cmdCtx.output)
		case "prepare:psql":
			prepareOpts := cmdCtx.prepareOptions(len(commands) > 1)
			if len(commands) == 1 {
				return r.deps.runPrepare(r.deps.stdout, r.deps.stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			}
			result, handled, err := r.deps.prepareResult(stdoutAndErr{stdout: r.deps.stdout, stderr: r.deps.stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			prepared = &result
		case "prepare:lb":
			prepareOpts := cmdCtx.prepareOptions(len(commands) > 1)
			if len(commands) == 1 {
				return r.deps.runPrepareLB(r.deps.stdout, r.deps.stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			}
			result, handled, err := r.deps.prepareResultLB(stdoutAndErr{stdout: r.deps.stdout, stderr: r.deps.stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			prepared = &result
		case "plan:psql":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			return r.deps.runPlan(r.deps.stdout, r.deps.stderr, cmdCtx.prepareOptions(false), cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args, cmdCtx.output, "psql")
		case "plan:lb":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			return r.deps.runPlan(r.deps.stdout, r.deps.stderr, cmdCtx.prepareOptions(false), cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args, cmdCtx.output, "lb")
		case "run:psql":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer r.deps.cleanupPreparedInstance(context.Background(), r.deps.stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			if err := r.deps.runRun(r.deps.stdout, r.deps.stderr, runOpts, "psql", cmd.Args, cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
				return err
			}
		case "run:pgbench":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer r.deps.cleanupPreparedInstance(context.Background(), r.deps.stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			if err := r.deps.runRun(r.deps.stdout, r.deps.stderr, runOpts, "pgbench", cmd.Args, cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
				return err
			}
		case "status":
			if len(commands) > 1 {
				return fmt.Errorf("status cannot be combined with other commands")
			}
			return r.deps.runStatus(r.deps.stdout, cmdCtx.statusOptions(), cmdCtx.workspaceRoot, cmdCtx.output, cmd.Args)
		case "watch":
			if len(commands) > 1 {
				return fmt.Errorf("watch cannot be combined with other commands")
			}
			return r.deps.runWatch(r.deps.stdout, cmdCtx.prepareOptions(false), cmd.Args)
		case "config":
			if len(commands) > 1 {
				return fmt.Errorf("config cannot be combined with other commands")
			}
			return r.deps.runConfig(r.deps.stdout, cmdCtx.configOptions(), cmd.Args, cmdCtx.output)
		case "prepare":
			invocation, showHelp, err := parsePrepareAliasArgs(cmd.Args)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintPrepareUsage(r.deps.stdout)
				return nil
			}
			alias, aliasPath, ref, err := resolvePrepareAliasWithOptionalRef(cmdCtx.workspaceRoot, cmdCtx.cwd, invocation.Ref, invocation.GitRef, invocation.RefMode, invocation.RefKeepWorktree)
			if err != nil {
				return err
			}
			alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)
			prepareOpts := cmdCtx.prepareOptions(len(commands) > 1)
			parsed := prepareArgs{
				Image:          alias.Image,
				PsqlArgs:       alias.Args,
				Watch:          invocation.Watch,
				WatchSpecified: invocation.WatchSpecified,
			}
			switch alias.Kind {
			case "psql":
				if len(commands) == 1 {
					return runPrepareParsed(r.deps.stdout, r.deps.stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, parsed, ref)
				}
				result, handled, err := prepareResultParsed(stdoutAndErr{stdout: r.deps.stdout, stderr: r.deps.stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, parsed, ref)
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
				prepared = &result
			case "lb":
				if len(commands) == 1 {
					return runPrepareLiquibaseParsedWithPathMode(r.deps.stdout, r.deps.stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, filepath.Dir(aliasPath), parsed, ref, false)
				}
				result, handled, err := prepareResultLiquibaseParsedWithPathMode(stdoutAndErr{stdout: r.deps.stdout, stderr: r.deps.stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, filepath.Dir(aliasPath), parsed, ref, false)
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
				prepared = &result
			default:
				return fmt.Errorf("unknown prepare alias kind: %s", alias.Kind)
			}
		case "plan":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			invocation, showHelp, err := parsePlanAliasArgs(cmd.Args)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintPlanUsage(r.deps.stdout)
				return nil
			}
			alias, aliasPath, ref, err := resolvePrepareAliasWithOptionalRef(cmdCtx.workspaceRoot, cmdCtx.cwd, invocation.Ref, invocation.GitRef, invocation.RefMode, invocation.RefKeepWorktree)
			if err != nil {
				return err
			}
			alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)
			return runPlanKindParsedWithPathMode(
				r.deps.stdout,
				r.deps.stderr,
				cmdCtx.prepareOptions(false),
				cmdCtx.cfgResult,
				cmdCtx.workspaceRoot,
				filepath.Dir(aliasPath),
				prepareArgs{Image: alias.Image, PsqlArgs: alias.Args},
				ref,
				cmdCtx.output,
				alias.Kind,
				alias.Kind != "lb",
			)
		case "run":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer r.deps.cleanupPreparedInstance(context.Background(), r.deps.stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			invocation, showHelp, err := parseRunAliasArgs(cmd.Args, prepared == nil)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintRunUsage(r.deps.stdout)
				return nil
			}
			alias, aliasPath, err := resolveRunAliasDefinition(cmdCtx.workspaceRoot, cmdCtx.cwd, invocation.Ref)
			if err != nil {
				return err
			}
			alias.Args = rebaseRunAliasArgs(alias.Kind, alias.Args, aliasPath)
			if err := r.deps.runRun(r.deps.stdout, r.deps.stderr, runOpts, alias.Kind, buildRunAliasCommandArgs(alias, invocation), cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
				return err
			}
		default:
			if strings.HasPrefix(cmd.Name, "prepare:") {
				kind := strings.TrimSpace(strings.TrimPrefix(cmd.Name, "prepare:"))
				if kind == "" {
					return fmt.Errorf("missing prepare kind (consider prepare:psql)")
				}
				return fmt.Errorf("unknown prepare kind: %s", kind)
			}
			if strings.HasPrefix(cmd.Name, "plan:") {
				kind := strings.TrimSpace(strings.TrimPrefix(cmd.Name, "plan:"))
				if kind == "" {
					return fmt.Errorf("missing plan kind (consider plan:psql)")
				}
				return fmt.Errorf("unknown plan kind: %s", kind)
			}
			if strings.HasPrefix(cmd.Name, "run:") {
				kind := strings.TrimSpace(strings.TrimPrefix(cmd.Name, "run:"))
				if kind == "" {
					return fmt.Errorf("missing run kind (consider run:psql)")
				}
				return fmt.Errorf("unknown run kind: %s", kind)
			}
			return fmt.Errorf("unknown command: %s", cmd.Name)
		}
	}
	return nil
}

func prepareStageUsesRef(cmd cli.Command) bool {
	if cmd.Name != "prepare" && !strings.HasPrefix(cmd.Name, "prepare:") {
		return false
	}
	for _, arg := range cmd.Args {
		switch strings.TrimSpace(arg) {
		case "--ref", "--ref-mode", "--ref-keep-worktree":
			return true
		}
	}
	return false
}
