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
	PlanOnly    bool
}

func RunPrepare(ctx context.Context, opts PrepareOptions) (client.PrepareJobResult, error) {
	if opts.PlanOnly {
		return client.PrepareJobResult{}, fmt.Errorf("plan-only is not supported by RunPrepare")
	}
	cliClient, err := prepareClient(ctx, opts)
	if err != nil {
		return client.PrepareJobResult{}, err
	}

	prepareKind := strings.TrimSpace(opts.PrepareKind)
	if prepareKind == "" {
		prepareKind = "psql"
	}

	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "submitting prepare job")
	}
	accepted, err := cliClient.CreatePrepareJob(ctx, client.PrepareJobRequest{
		PrepareKind: prepareKind,
		ImageID:     opts.ImageID,
		PsqlArgs:    opts.PsqlArgs,
		Stdin:       opts.Stdin,
		PlanOnly:    false,
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

	status, err := waitForPrepare(ctx, cliClient, jobID)
	if err != nil {
		return client.PrepareJobResult{}, err
	}
	if status.Result == nil {
		return client.PrepareJobResult{}, fmt.Errorf("prepare job succeeded without result")
	}
	return *status.Result, nil
}

type PlanResult struct {
	PrepareKind           string            `json:"prepare_kind"`
	ImageID               string            `json:"image_id"`
	PrepareArgsNormalized string            `json:"prepare_args_normalized"`
	Tasks                 []client.PlanTask `json:"tasks"`
}

func RunPlan(ctx context.Context, opts PrepareOptions) (PlanResult, error) {
	opts.PlanOnly = true
	cliClient, err := prepareClient(ctx, opts)
	if err != nil {
		return PlanResult{}, err
	}

	prepareKind := strings.TrimSpace(opts.PrepareKind)
	if prepareKind == "" {
		prepareKind = "psql"
	}

	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "submitting prepare job (plan-only)")
	}
	accepted, err := cliClient.CreatePrepareJob(ctx, client.PrepareJobRequest{
		PrepareKind: prepareKind,
		ImageID:     opts.ImageID,
		PsqlArgs:    opts.PsqlArgs,
		Stdin:       opts.Stdin,
		PlanOnly:    true,
	})
	if err != nil {
		return PlanResult{}, err
	}

	jobID := accepted.JobID
	if jobID == "" {
		return PlanResult{}, fmt.Errorf("prepare job id missing")
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "waiting for prepare job %s\n", jobID)
	}

	status, err := waitForPrepare(ctx, cliClient, jobID)
	if err != nil {
		return PlanResult{}, err
	}
	if !status.PlanOnly {
		return PlanResult{}, fmt.Errorf("prepare job is not plan-only")
	}
	if len(status.Tasks) == 0 {
		return PlanResult{}, fmt.Errorf("plan job succeeded without tasks")
	}
	return PlanResult{
		PrepareKind:           status.PrepareKind,
		ImageID:               status.ImageID,
		PrepareArgsNormalized: status.PrepareArgsNormalized,
		Tasks:                 status.Tasks,
	}, nil
}

func prepareClient(ctx context.Context, opts PrepareOptions) (*client.Client, error) {
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
				return nil, err
			}
			endpoint = resolved.Endpoint
			authToken = resolved.AuthToken
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return nil, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: authToken})
	return cliClient, nil
}

func waitForPrepare(ctx context.Context, cliClient *client.Client, jobID string) (client.PrepareJobStatus, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, found, err := cliClient.GetPrepareJob(ctx, jobID)
		if err != nil {
			return client.PrepareJobStatus{}, err
		}
		if !found {
			return client.PrepareJobStatus{}, fmt.Errorf("prepare job not found: %s", jobID)
		}
		switch status.Status {
		case "succeeded":
			return status, nil
		case "failed":
			if status.Error != nil {
				if status.Error.Details != "" {
					return client.PrepareJobStatus{}, fmt.Errorf("%s: %s", status.Error.Message, status.Error.Details)
				}
				return client.PrepareJobStatus{}, fmt.Errorf("%s", status.Error.Message)
			}
			return client.PrepareJobStatus{}, fmt.Errorf("prepare job failed")
		}

		select {
		case <-ctx.Done():
			return client.PrepareJobStatus{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
