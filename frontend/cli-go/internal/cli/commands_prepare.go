package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

type PrepareOptions struct {
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

	ImageID     string
	PsqlArgs    []string
	Stdin       *string
	PrepareKind string
}

func RunPrepare(ctx context.Context, opts PrepareOptions) (client.PrepareJobResult, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	endpoint := strings.TrimSpace(opts.Endpoint)
	authToken := ""

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
				return client.PrepareJobResult{}, err
			}
			endpoint = resolved.Endpoint
			authToken = resolved.AuthToken
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return client.PrepareJobResult{}, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	prepareKind := strings.TrimSpace(opts.PrepareKind)
	if prepareKind == "" {
		prepareKind = "psql"
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: authToken})
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "submitting prepare job")
	}
	accepted, err := cliClient.CreatePrepareJob(ctx, client.PrepareJobRequest{
		PrepareKind: prepareKind,
		ImageID:     opts.ImageID,
		PsqlArgs:    opts.PsqlArgs,
		Stdin:       opts.Stdin,
	})
	if err != nil {
		return client.PrepareJobResult{}, err
	}

	jobID := accepted.JobID
	if jobID == "" {
		return client.PrepareJobResult{}, fmt.Errorf("prepare job id missing")
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "waiting for prepare job %s\n", jobID)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, found, err := cliClient.GetPrepareJob(ctx, jobID)
		if err != nil {
			return client.PrepareJobResult{}, err
		}
		if !found {
			return client.PrepareJobResult{}, fmt.Errorf("prepare job not found: %s", jobID)
		}
		switch status.Status {
		case "succeeded":
			if status.Result == nil {
				return client.PrepareJobResult{}, fmt.Errorf("prepare job succeeded without result")
			}
			return *status.Result, nil
		case "failed":
			if status.Error != nil {
				if status.Error.Details != "" {
					return client.PrepareJobResult{}, fmt.Errorf("%s: %s", status.Error.Message, status.Error.Details)
				}
				return client.PrepareJobResult{}, fmt.Errorf("%s", status.Error.Message)
			}
			return client.PrepareJobResult{}, fmt.Errorf("prepare job failed")
		}

		select {
		case <-ctx.Done():
			return client.PrepareJobResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
