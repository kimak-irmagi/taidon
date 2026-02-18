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

var connectOrStart = daemon.ConnectOrStart

type ConfigOptions struct {
	ProfileName     string
	Mode            string
	AuthToken       string
	Endpoint        string
	Autostart       bool
	DaemonPath      string
	RunDir          string
	StateDir        string
	EngineRunDir    string
	EngineStatePath string
	EngineStoreDir  string
	WSLVHDXPath     string
	WSLMountUnit    string
	WSLMountFSType  string
	WSLDistro       string
	Timeout         time.Duration
	IdleTimeout     time.Duration
	StartupTimeout  time.Duration
	Verbose         bool

	Path      string
	Value     any
	Effective bool
}

func RunConfigGet(ctx context.Context, opts ConfigOptions) (any, error) {
	cliClient, err := configClient(ctx, opts)
	if err != nil {
		return nil, err
	}
	return cliClient.GetConfig(ctx, opts.Path, opts.Effective)
}

func RunConfigSet(ctx context.Context, opts ConfigOptions) (client.ConfigValue, error) {
	cliClient, err := configClient(ctx, opts)
	if err != nil {
		return client.ConfigValue{}, err
	}
	return cliClient.SetConfig(ctx, client.ConfigValue{
		Path:  opts.Path,
		Value: opts.Value,
	})
}

func RunConfigRemove(ctx context.Context, opts ConfigOptions) (client.ConfigValue, error) {
	cliClient, err := configClient(ctx, opts)
	if err != nil {
		return client.ConfigValue{}, err
	}
	return cliClient.RemoveConfig(ctx, opts.Path)
}

func RunConfigSchema(ctx context.Context, opts ConfigOptions) (any, error) {
	cliClient, err := configClient(ctx, opts)
	if err != nil {
		return nil, err
	}
	return cliClient.GetConfigSchema(ctx)
}

func configClient(ctx context.Context, opts ConfigOptions) (*client.Client, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	endpoint := strings.TrimSpace(opts.Endpoint)
	authToken := strings.TrimSpace(opts.AuthToken)

	if mode == "local" {
		authToken = ""
		if endpoint == "" {
			endpoint = "auto"
		}
		if endpoint == "auto" {
			if opts.Verbose {
				fmt.Fprintln(os.Stderr, "checking local engine state")
			}
			resolved, err := connectOrStart(ctx, daemon.ConnectOptions{
				Endpoint:        endpoint,
				Autostart:       opts.Autostart,
				DaemonPath:      opts.DaemonPath,
				RunDir:          opts.RunDir,
				StateDir:        opts.StateDir,
				EngineRunDir:    opts.EngineRunDir,
				EngineStatePath: opts.EngineStatePath,
				EngineStoreDir:  opts.EngineStoreDir,
				WSLVHDXPath:     opts.WSLVHDXPath,
				WSLMountUnit:    opts.WSLMountUnit,
				WSLMountFSType:  opts.WSLMountFSType,
				WSLDistro:       opts.WSLDistro,
				IdleTimeout:     opts.IdleTimeout,
				StartupTimeout:  opts.StartupTimeout,
				ClientTimeout:   opts.Timeout,
				Verbose:         opts.Verbose,
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
