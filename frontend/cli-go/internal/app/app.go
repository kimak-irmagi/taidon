package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/paths"
	"github.com/sqlrs/cli/internal/wsl"
)

const defaultTimeout = 30 * time.Second
const defaultStartupTimeout = 5 * time.Second
const defaultIdleTimeout = 120 * time.Second

var parseArgsFn = cli.ParseArgs
var getwdFn = os.Getwd

func Run(args []string) error {
	opts, commands, err := parseArgsFn(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			cli.PrintUsage(os.Stdout)
			return nil
		}
		cli.PrintUsage(os.Stderr)
		return err
	}

	cwd, err := getwdFn()
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
		return runInit(os.Stdout, cwd, opts.Workspace, commands[0].Args, opts.Verbose)
	}

	if commands[0].Name == "diff" {
		if len(commands) > 1 {
			return fmt.Errorf("diff cannot be combined with other commands")
		}
		return runDiff(os.Stdout, os.Stderr, cwd, commands[0].Args, opts.Output, opts.Verbose)
	}

	cmdCtx, err := resolveCommandContext(cwd, opts)
	if err != nil {
		return err
	}

	var prepared *client.PrepareJobResult
	for _, cmd := range commands {
		switch cmd.Name {
		case "alias":
			if len(commands) > 1 {
				return fmt.Errorf("alias cannot be combined with other commands")
			}
			return runAliasCommand(os.Stdout, cmdCtx, cmd.Args)
		case "ls":
			if len(commands) > 1 {
				return fmt.Errorf("ls cannot be combined with other commands")
			}
			return runLs(os.Stdout, cmdCtx.lsOptions(), cmd.Args, cmdCtx.output)
		case "rm":
			if len(commands) > 1 {
				return fmt.Errorf("rm cannot be combined with other commands")
			}
			return runRm(os.Stdout, cmdCtx.rmOptions(), cmd.Args, cmdCtx.output)
		case "prepare:psql":
			prepareOpts := cmdCtx.prepareOptions(len(commands) > 1)
			if len(commands) == 1 {
				return runPrepare(os.Stdout, os.Stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			}
			result, handled, err := prepareResult(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
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
				return runPrepareLiquibase(os.Stdout, os.Stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
			}
			result, handled, err := prepareResultLiquibase(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args)
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
			return runPlanKind(os.Stdout, os.Stderr, cmdCtx.prepareOptions(false), cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args, cmdCtx.output, "psql")
		case "plan:lb":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			return runPlanKind(os.Stdout, os.Stderr, cmdCtx.prepareOptions(false), cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, cmd.Args, cmdCtx.output, "lb")
		case "run:psql":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer cleanupPreparedInstance(context.Background(), os.Stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			if err := runRun(os.Stdout, os.Stderr, runOpts, "psql", cmd.Args, cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
				return err
			}
		case "run:pgbench":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer cleanupPreparedInstance(context.Background(), os.Stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			if err := runRun(os.Stdout, os.Stderr, runOpts, "pgbench", cmd.Args, cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
				return err
			}
		case "status":
			if len(commands) > 1 {
				return fmt.Errorf("status cannot be combined with other commands")
			}
			return runStatus(os.Stdout, cmdCtx.statusOptions(), cmdCtx.workspaceRoot, cmdCtx.output, cmd.Args)
		case "watch":
			if len(commands) > 1 {
				return fmt.Errorf("watch cannot be combined with other commands")
			}
			return runWatch(os.Stdout, cmdCtx.prepareOptions(false), cmd.Args)
		case "config":
			if len(commands) > 1 {
				return fmt.Errorf("config cannot be combined with other commands")
			}
			return runConfig(os.Stdout, cmdCtx.configOptions(), cmd.Args, cmdCtx.output)
		case "prepare":
			invocation, showHelp, err := parsePrepareAliasArgs(cmd.Args)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintPrepareUsage(os.Stdout)
				return nil
			}
			aliasPath, err := resolvePrepareAliasPath(cmdCtx.workspaceRoot, cmdCtx.cwd, invocation.Ref)
			if err != nil {
				return err
			}
			alias, err := loadPrepareAlias(aliasPath)
			if err != nil {
				return err
			}
			alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)
			aliasArgs := buildPrepareAliasCommandArgs(alias, invocation)
			prepareOpts := cmdCtx.prepareOptions(len(commands) > 1)
			switch alias.Kind {
			case "psql":
				if len(commands) == 1 {
					return runPrepare(os.Stdout, os.Stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, aliasArgs)
				}
				result, handled, err := prepareResult(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, aliasArgs)
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
				prepared = &result
			case "lb":
				if len(commands) == 1 {
					return runPrepareLiquibaseWithPathMode(os.Stdout, os.Stderr, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, aliasArgs, false)
				}
				result, handled, err := prepareResultLiquibaseWithPathMode(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, prepareOpts, cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, aliasArgs, false)
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
			ref, showHelp, err := parsePlanAliasArgs(cmd.Args)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintPlanUsage(os.Stdout)
				return nil
			}
			aliasPath, err := resolvePrepareAliasPath(cmdCtx.workspaceRoot, cmdCtx.cwd, ref)
			if err != nil {
				return err
			}
			alias, err := loadPrepareAlias(aliasPath)
			if err != nil {
				return err
			}
			alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)
			return runPlanKindWithPathMode(os.Stdout, os.Stderr, cmdCtx.prepareOptions(false), cmdCtx.cfgResult, cmdCtx.workspaceRoot, cmdCtx.cwd, buildPlanAliasCommandArgs(alias), cmdCtx.output, alias.Kind, alias.Kind != "lb")
		case "run":
			runOpts := cmdCtx.runOptions()
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer cleanupPreparedInstance(context.Background(), os.Stderr, runOpts, prepared.InstanceID, cmdCtx.verbose)
			}
			invocation, showHelp, err := parseRunAliasArgs(cmd.Args, prepared == nil)
			if err != nil {
				return err
			}
			if showHelp {
				cli.PrintRunUsage(os.Stdout)
				return nil
			}
			aliasPath, err := resolveRunAliasPath(cmdCtx.workspaceRoot, cmdCtx.cwd, invocation.Ref)
			if err != nil {
				return err
			}
			alias, err := loadRunAlias(aliasPath)
			if err != nil {
				return err
			}
			alias.Args = rebaseRunAliasArgs(alias.Kind, alias.Args, aliasPath)
			if err := runRun(os.Stdout, os.Stderr, runOpts, alias.Kind, buildRunAliasCommandArgs(alias, invocation), cmdCtx.workspaceRoot, cmdCtx.cwd); err != nil {
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

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func formatCleanupResult(result client.DeleteResult) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(result.Outcome) != "" {
		parts = append(parts, "outcome="+result.Outcome)
	}
	if strings.TrimSpace(result.Root.Blocked) != "" {
		parts = append(parts, "blocked="+result.Root.Blocked)
	}
	if result.Root.Connections != nil {
		parts = append(parts, fmt.Sprintf("connections=%d", *result.Root.Connections))
	}
	if len(parts) == 0 {
		return "blocked"
	}
	return strings.Join(parts, ", ")
}

func startCleanupSpinner(instanceID string, verbose bool) func() {
	label := fmt.Sprintf("Deleting instance %s", instanceID)
	out := os.Stdout
	if verbose || !isTerminalWriterFn(out) {
		fmt.Fprintln(out, label)
		return func() {}
	}

	clearLen := len(label) + 2
	done := make(chan struct{})
	shown := make(chan struct{})
	go func() {
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			close(shown)
		case <-done:
			return
		}
		spinner := []string{"-", "\\", "|", "/"}
		idx := 0
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				clearLineOut(out, clearLen)
				return
			case <-ticker.C:
				clearLineOut(out, clearLen)
				fmt.Fprintf(out, "%s %s", label, spinner[idx])
				idx = (idx + 1) % len(spinner)
			}
		}
	}()
	return func() {
		close(done)
		select {
		case <-shown:
			clearLineOut(out, clearLen)
		default:
		}
	}
}

func clearLineOut(out io.Writer, width int) {
	if out == nil {
		return
	}
	if width <= 0 {
		width = 1
	}
	fmt.Fprint(out, "\r")
	fmt.Fprint(out, strings.Repeat(" ", width))
	fmt.Fprint(out, "\r")
}

func resolveWSLSettings(cfg config.Config, dirs paths.Dirs, daemonPath string) (string, string, string, string, string, string, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Engine.WSL.Mode))
	if mode == "" {
		return daemonPath, "", "", "", "", "", "", nil
	}
	if mode != "auto" && mode != "required" {
		return daemonPath, "", "", "", "", "", "", nil
	}

	stateDir := strings.TrimSpace(cfg.Engine.WSL.StateDir)
	distro := strings.TrimSpace(cfg.Engine.WSL.Distro)
	mountUnit := strings.TrimSpace(cfg.Engine.WSL.Mount.Unit)
	mountFSType := strings.TrimSpace(cfg.Engine.WSL.Mount.FSType)
	if distro == "" {
		distros, err := listWSLDistrosFn()
		if err != nil {
			if mode == "required" {
				return "", "", "", "", "", "", "", fmt.Errorf("WSL unavailable: %v", err)
			}
			return daemonPath, "", "", "", "", "", "", nil
		}
		distro, err = wsl.SelectDistro(distros, "")
		if err != nil {
			if mode == "required" {
				return "", "", "", "", "", "", "", fmt.Errorf("WSL distro resolution failed: %v", err)
			}
			return daemonPath, "", "", "", "", "", "", nil
		}
	}
	if distro == "" || stateDir == "" {
		if mode == "required" {
			return "", "", "", "", "", "", "", fmt.Errorf("WSL configuration is missing distro or stateDir")
		}
		return daemonPath, "", "", "", "", "", "", nil
	}
	if mountUnit == "" && mode == "required" {
		return "", "", "", "", "", "", "", fmt.Errorf("WSL configuration is missing mount unit (run sqlrs init local --snapshot btrfs)")
	}
	if mountFSType == "" && mountUnit != "" {
		mountFSType = "btrfs"
	}

	engineBinary := daemonPath
	if cfg.Engine.WSL.EnginePath != "" {
		engineBinary = cfg.Engine.WSL.EnginePath
	}
	wslDaemonPath, err := windowsToWSLPath(engineBinary)
	if err != nil {
		if mode == "required" {
			return "", "", "", "", "", "", "", err
		}
		return daemonPath, "", "", "", "", "", "", nil
	}

	statePath := filepath.Join(dirs.StateDir, "engine.json")
	wslStatePath, err := windowsToWSLPath(statePath)
	if err != nil {
		if mode == "required" {
			return "", "", "", "", "", "", "", err
		}
		return daemonPath, "", "", "", "", "", "", nil
	}

	runDir := path.Join(stateDir, "run")
	return wslDaemonPath, runDir, wslStatePath, stateDir, distro, mountUnit, mountFSType, nil
}

func resolveAuthToken(auth config.AuthConfig) string {
	if env := strings.TrimSpace(auth.TokenEnv); env != "" {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(auth.Token)
}

func windowsToWSLPath(value string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(cleaned, "/") {
		return cleaned, nil
	}
	drive := ""
	rest := ""
	if len(cleaned) >= 2 && cleaned[1] == ':' {
		letter := cleaned[0]
		if (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') {
			drive = strings.ToLower(cleaned[:1])
			rest = cleaned[2:]
		}
	}
	if drive == "" {
		vol := filepath.VolumeName(cleaned)
		if vol == "" {
			return "", fmt.Errorf("path is not absolute: %s", cleaned)
		}
		drive = strings.TrimSuffix(strings.ToLower(vol), ":")
		rest = cleaned[len(vol):]
	}
	rest = strings.TrimLeft(rest, `\\/`)
	rest = strings.ReplaceAll(rest, "\\", "/")
	if rest == "" {
		return fmt.Sprintf("/mnt/%s", drive), nil
	}
	return fmt.Sprintf("/mnt/%s/%s", drive, rest), nil
}
