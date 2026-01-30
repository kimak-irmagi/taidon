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

func Run(args []string) error {
	opts, commands, err := cli.ParseArgs(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			cli.PrintUsage(os.Stdout)
			return nil
		}
		cli.PrintUsage(os.Stderr)
		return err
	}

	cwd, err := os.Getwd()
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
	engineWSLMountDevice := ""
	engineWSLMountFSType := ""
	wslDistro := ""
	if runtime.GOOS == "windows" {
		daemonPath, engineRunDir, engineStatePath, engineStoreDir, wslDistro, engineWSLMountDevice, engineWSLMountFSType, err = resolveWSLSettings(cfg, dirs, daemonPath)
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
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
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
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
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
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
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
		case "plan:psql":
			if len(commands) > 1 {
				return fmt.Errorf("plan cannot be combined with other commands")
			}
			runOpts := cli.PrepareOptions{
				ProfileName:     profileName,
				Mode:            mode,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			return runPlan(os.Stdout, os.Stderr, runOpts, cfgResult, workspaceRoot, cwd, cmd.Args, output)
		case "run:psql":
			runOpts := cli.RunOptions{
				ProfileName:     profileName,
				Mode:            mode,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer func(instanceID string) {
					_ = cli.DeleteInstance(context.Background(), runOpts, instanceID)
				}(prepared.InstanceID)
			}
			if err := runRun(os.Stdout, os.Stderr, runOpts, "psql", cmd.Args, workspaceRoot, cwd); err != nil {
				return err
			}
		case "run:pgbench":
			runOpts := cli.RunOptions{
				ProfileName:     profileName,
				Mode:            mode,
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
				WSLMountFSType:  engineWSLMountFSType,
				WSLDistro:       wslDistro,
				Timeout:         timeout,
				StartupTimeout:  startupTimeout,
				Verbose:         opts.Verbose,
			}
			if prepared != nil {
				runOpts.InstanceRef = prepared.InstanceID
				defer func(instanceID string) {
					_ = cli.DeleteInstance(context.Background(), runOpts, instanceID)
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
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
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
				Endpoint:        profile.Endpoint,
				Autostart:       profile.Autostart,
				DaemonPath:      daemonPath,
				RunDir:          runDir,
				StateDir:        dirs.StateDir,
				EngineRunDir:    engineRunDir,
				EngineStatePath: engineStatePath,
				EngineStoreDir:  engineStoreDir,
				WSLMountDevice:  engineWSLMountDevice,
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
	mountDevice := strings.TrimSpace(cfg.Engine.WSL.Mount.Device)
	mountFSType := strings.TrimSpace(cfg.Engine.WSL.Mount.FSType)
	if distro == "" {
		distros, err := listWSLDistros()
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
	return wslDaemonPath, runDir, wslStatePath, stateDir, distro, mountDevice, mountFSType, nil
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
