package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
	"sqlrs/cli/internal/wsl"
)

const defaultTimeout = 30 * time.Second
const defaultStartupTimeout = 5 * time.Second

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

	cfgResult, err := config.Load(config.LoadOptions{WorkingDir: cwd})
	if err != nil {
		return err
	}

	cfg := cfgResult.Config
	dirs := cfgResult.Paths
	workspaceRoot := ""
	if cfgResult.ProjectConfigPath != "" {
		workspaceRoot = filepath.Dir(filepath.Dir(cfgResult.ProjectConfigPath))
	}

	profileName := opts.Profile
	if profileName == "" {
		profileName = cfg.DefaultProfile
	}
	if profileName == "" {
		profileName = "local"
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile not found: %s", profileName)
	}

	if opts.Endpoint != "" {
		profile.Endpoint = opts.Endpoint
	}

	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" || mode == "auto" {
		mode = strings.ToLower(strings.TrimSpace(profile.Mode))
		if mode == "" {
			mode = "local"
		}
	}

	if mode != "local" && mode != "remote" {
		return fmt.Errorf("invalid mode: %s", mode)
	}

	authToken := resolveAuthToken(profile.Auth)

	output := strings.ToLower(strings.TrimSpace(cfg.Client.Output))
	if opts.Output != "" {
		output = strings.ToLower(strings.TrimSpace(opts.Output))
	}
	if output == "" {
		output = "human"
	}
	if output != "human" && output != "json" {
		return fmt.Errorf("invalid output: %s", output)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		parsed, err := config.ParseDuration(cfg.Client.Timeout, defaultTimeout)
		if err != nil {
			return err
		}
		timeout = parsed
	}

	startupTimeout, err := config.ParseDuration(cfg.Orchestrator.StartupTimeout, defaultStartupTimeout)
	if err != nil {
		return err
	}

	runDir := cfg.Orchestrator.RunDir
	if runDir == "" {
		runDir = filepath.Join(dirs.StateDir, "run")
	}

	daemonPath := os.Getenv("SQLRS_DAEMON_PATH")
	if daemonPath == "" {
		daemonPath = cfg.Orchestrator.DaemonPath
	}
	engineRunDir := ""
	engineStatePath := ""
	engineStoreDir := strings.TrimSpace(cfg.Engine.StorePath)
	engineHostStorePath := engineStoreDir
	engineWSLMountUnit := ""
	engineWSLMountFSType := ""
	wslDistro := ""
	if runtime.GOOS == "windows" {
		daemonPath, engineRunDir, engineStatePath, engineStoreDir, wslDistro, engineWSLMountUnit, engineWSLMountFSType, err = resolveWSLSettings(cfg, dirs, daemonPath)
		if err != nil {
			return err
		}
	}

	var prepared *client.PrepareJobResult
	for _, cmd := range commands {
		switch cmd.Name {
		case "ls":
			if len(commands) > 1 {
				return fmt.Errorf("ls cannot be combined with other commands")
			}
			runOpts := cli.LsOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runLs(os.Stdout, runOpts, cmd.Args, output)
		case "rm":
			if len(commands) > 1 {
				return fmt.Errorf("rm cannot be combined with other commands")
			}
			runOpts := cli.RmOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runRm(os.Stdout, runOpts, cmd.Args, output)
		case "prepare:psql":
			runOpts := cli.PrepareOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if len(commands) == 1 {
				return runPrepare(os.Stdout, os.Stderr, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args)
			}
			result, handled, err := prepareResult(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			prepared = &result
		case "prepare:lb":
			runOpts := cli.PrepareOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if len(commands) == 1 {
				return runPrepareLiquibase(os.Stdout, os.Stderr, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args)
			}
			result, handled, err := prepareResultLiquibase(stdoutAndErr{stdout: os.Stdout, stderr: os.Stderr}, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args)
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
			runOpts := cli.PrepareOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runPlanKind(os.Stdout, os.Stderr, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args, output, "psql")
		case "plan:lb":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			runOpts := cli.PrepareOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runPlanKind(os.Stdout, os.Stderr, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args, output, "lb")
		case "run:psql":
			runOpts := cli.RunOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer func(instanceID string) {
					stopSpinner := startCleanupSpinner(instanceID, opts.Verbose)
					result, status, err := cli.DeleteInstanceDetailed(context.Background(), runOpts, instanceID)
					stopSpinner()
					if err != nil {
						if opts.Verbose {
							fmt.Fprintf(os.Stderr, "cleanup failed for instance %s: %v\n", instanceID, err)
						} else {
							fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
						}
						return
					}
					if status == http.StatusConflict || strings.EqualFold(result.Outcome, "blocked") {
						if opts.Verbose {
							fmt.Fprintf(os.Stderr, "cleanup blocked for instance %s: %s\n", instanceID, formatCleanupResult(result))
						} else {
							fmt.Fprintf(os.Stderr, "cleanup blocked for instance %s\n", instanceID)
						}
					}
				}(prepared.InstanceID)
			}
			if err := runRun(os.Stdout, os.Stderr, runOpts, "psql", cmd.Args, workspaceRoot, cwd); err != nil {
				return err
			}
		case "run:pgbench":
			runOpts := cli.RunOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer func(instanceID string) {
					stopSpinner := startCleanupSpinner(instanceID, opts.Verbose)
					result, status, err := cli.DeleteInstanceDetailed(context.Background(), runOpts, instanceID)
					stopSpinner()
					if err != nil {
						if opts.Verbose {
							fmt.Fprintf(os.Stderr, "cleanup failed for instance %s: %v\n", instanceID, err)
						} else {
							fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
						}
						return
					}
					if status == http.StatusConflict || strings.EqualFold(result.Outcome, "blocked") {
						if opts.Verbose {
							fmt.Fprintf(os.Stderr, "cleanup blocked for instance %s: %s\n", instanceID, formatCleanupResult(result))
						} else {
							fmt.Fprintf(os.Stderr, "cleanup blocked for instance %s\n", instanceID)
						}
					}
				}(prepared.InstanceID)
			}
			if err := runRun(os.Stdout, os.Stderr, runOpts, "pgbench", cmd.Args, workspaceRoot, cwd); err != nil {
				return err
			}
		case "status":
			if len(commands) > 1 {
				return fmt.Errorf("status cannot be combined with other commands")
			}
			if len(cmd.Args) > 0 {
				return fmt.Errorf("status does not accept arguments")
			}
			statusOpts := cli.StatusOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}

			result, err := cli.RunStatus(context.Background(), statusOpts)
			if err != nil {
				return err
			}

			result.Client = Version
			result.Workspace = workspaceRoot

			if output == "json" {
				if err := writeJSON(os.Stdout, result); err != nil {
					return err
				}
			} else {
				cli.PrintStatus(os.Stdout, result)
			}

			if !result.OK {
				return fmt.Errorf("service unhealthy")
			}
			return nil
		case "config":
			if len(commands) > 1 {
				return fmt.Errorf("config cannot be combined with other commands")
			}
			runOpts := cli.ConfigOptions{
				ProfileName:     profileName,
				Mode:            mode,
				AuthToken:       authToken,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLVHDXPath:     engineHostStorePath,
				WSLMountUnit:    engineWSLMountUnit,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runConfig(os.Stdout, runOpts, cmd.Args, output)
		case "prepare":
			return fmt.Errorf("missing prepare kind (consider prepare:psql)")
		case "plan":
			return fmt.Errorf("missing plan kind (consider plan:psql)")
		case "run":
			return fmt.Errorf("missing run kind (consider run:psql)")
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
	if verbose || !isTerminalWriter(out) {
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
	vol := filepath.VolumeName(cleaned)
	if vol == "" {
		return "", fmt.Errorf("path is not absolute: %s", cleaned)
	}
	drive := strings.TrimSuffix(strings.ToLower(vol), ":")
	rest := strings.TrimPrefix(cleaned[len(vol):], string(filepath.Separator))
	rest = strings.ReplaceAll(rest, "\\", "/")
	if rest == "" {
		return fmt.Sprintf("/mnt/%s", drive), nil
	}
	return fmt.Sprintf("/mnt/%s/%s", drive, rest), nil
}
