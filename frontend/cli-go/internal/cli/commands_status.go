package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

type StatusOptions struct {
	ProfileName    string
	Mode           string
	Endpoint       string
	Autostart      bool
	DaemonPath     string
	RunDir         string
	StateDir       string
	Timeout        time.Duration
	StartupTimeout time.Duration
	Verbose        bool
}

type StatusResult struct {
	OK         bool   `json:"ok"`
	Endpoint   string `json:"endpoint"`
	Profile    string `json:"profile"`
	Mode       string `json:"mode"`
	Client     string `json:"clientVersion,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	Version    string `json:"version,omitempty"`
	InstanceID string `json:"instanceId,omitempty"`
	PID        int    `json:"pid,omitempty"`
}

func RunStatus(ctx context.Context, opts StatusOptions) (StatusResult, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	endpoint := strings.TrimSpace(opts.Endpoint)

	if mode == "local" {
		if endpoint == "" {
			endpoint = "auto"
		}
		if endpoint == "auto" {
			if opts.Verbose {
				fmt.Fprintln(os.Stderr, "checking local engine state")
			}
			resolved, err := daemon.ConnectOrStart(ctx, daemon.ConnectOptions{
				Endpoint:       endpoint,
				Autostart:      opts.Autostart,
				DaemonPath:     opts.DaemonPath,
				RunDir:         opts.RunDir,
				StateDir:       opts.StateDir,
				StartupTimeout: opts.StartupTimeout,
				ClientTimeout:  opts.Timeout,
				Verbose:        opts.Verbose,
			})
			if err != nil {
				return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, err
			}
			endpoint = resolved.Endpoint
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout})
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "requesting health")
	}
	health, err := cliClient.Health(ctx)
	if err != nil {
		return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, err
	}

	return StatusResult{
		OK:         health.Ok,
		Endpoint:   endpoint,
		Profile:    opts.ProfileName,
		Mode:       mode,
		Version:    health.Version,
		InstanceID: health.InstanceID,
		PID:        health.PID,
	}, nil
}

func PrintStatus(w io.Writer, result StatusResult) {
	status := "unavailable"
	if result.OK {
		status = "ok"
	}

	fmt.Fprintf(w, "status: %s\n", status)
	fmt.Fprintf(w, "endpoint: %s\n", result.Endpoint)
	fmt.Fprintf(w, "profile: %s\n", result.Profile)
	fmt.Fprintf(w, "mode: %s\n", result.Mode)
	if result.Client != "" {
		fmt.Fprintf(w, "clientVersion: %s\n", result.Client)
	}
	if result.Workspace != "" {
		fmt.Fprintf(w, "workspace: %s\n", result.Workspace)
	} else {
		fmt.Fprintln(w, "workspace: (none)")
	}

	if result.Version != "" {
		fmt.Fprintf(w, "version: %s\n", result.Version)
	}
	if result.InstanceID != "" {
		fmt.Fprintf(w, "instanceId: %s\n", result.InstanceID)
	}
	if result.PID != 0 {
		fmt.Fprintf(w, "pid: %d\n", result.PID)
	}
}
