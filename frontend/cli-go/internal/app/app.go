package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

const defaultTimeout = 30 * time.Second
const defaultStartupTimeout = 5 * time.Second

func Run(args []string) error {
	opts, cmd, err := cli.ParseArgs(args)
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

	if cmd.Name == "init" {
		return runInit(os.Stdout, cwd, opts.Workspace, cmd.Args)
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

	switch cmd.Name {
	case "ls":
		runOpts := cli.LsOptions{
			ProfileName:    profileName,
			Mode:           mode,
			Endpoint:       profile.Endpoint,
			Autostart:      profile.Autostart,
			DaemonPath:     daemonPath,
			RunDir:         runDir,
			StateDir:       dirs.StateDir,
			Timeout:        timeout,
			StartupTimeout: startupTimeout,
			Verbose:        opts.Verbose,
		}
		return runLs(os.Stdout, runOpts, cmd.Args, output)
	case "rm":
		runOpts := cli.RmOptions{
			ProfileName:    profileName,
			Mode:           mode,
			Endpoint:       profile.Endpoint,
			Autostart:      profile.Autostart,
			DaemonPath:     daemonPath,
			RunDir:         runDir,
			StateDir:       dirs.StateDir,
			Timeout:        timeout,
			StartupTimeout: startupTimeout,
			Verbose:        opts.Verbose,
		}
		return runRm(os.Stdout, runOpts, cmd.Args, output)
	case "prepare:psql":
		runOpts := cli.PrepareOptions{
			ProfileName:    profileName,
			Mode:           mode,
			Endpoint:       profile.Endpoint,
			Autostart:      profile.Autostart,
			DaemonPath:     daemonPath,
			RunDir:         runDir,
			StateDir:       dirs.StateDir,
			Timeout:        timeout,
			StartupTimeout: startupTimeout,
			Verbose:        opts.Verbose,
		}
		return runPrepare(os.Stdout, os.Stderr, runOpts, cfgResult, cwd, cmd.Args)
	case "status":
		if len(cmd.Args) > 0 {
			return fmt.Errorf("status does not accept arguments")
		}
		statusOpts := cli.StatusOptions{
			ProfileName:    profileName,
			Mode:           mode,
			Endpoint:       profile.Endpoint,
			Autostart:      profile.Autostart,
			DaemonPath:     daemonPath,
			RunDir:         runDir,
			StateDir:       dirs.StateDir,
			Timeout:        timeout,
			StartupTimeout: startupTimeout,
			Verbose:        opts.Verbose,
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
	case "prepare":
		return fmt.Errorf("missing prepare kind (consider prepare:psql)")
	default:
		if strings.HasPrefix(cmd.Name, "prepare:") {
			kind := strings.TrimSpace(strings.TrimPrefix(cmd.Name, "prepare:"))
			if kind == "" {
				return fmt.Errorf("missing prepare kind (consider prepare:psql)")
			}
			return fmt.Errorf("unknown prepare kind: %s", kind)
		}
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
