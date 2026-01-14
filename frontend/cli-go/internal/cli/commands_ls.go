package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

type LsOptions struct {
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

	IncludeNames     bool
	IncludeInstances bool
	IncludeStates    bool

	FilterName     string
	FilterInstance string
	FilterState    string
	FilterKind     string
	FilterImage    string

	Quiet    bool
	NoHeader bool
}

type LsResult struct {
	Names     *[]client.NameEntry     `json:"names,omitempty"`
	Instances *[]client.InstanceEntry `json:"instances,omitempty"`
	States    *[]client.StateEntry    `json:"states,omitempty"`
}

type LsPrintOptions struct {
	Quiet    bool
	NoHeader bool
}

func RunLs(ctx context.Context, opts LsOptions) (LsResult, error) {
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
				return LsResult{}, err
			}
			endpoint = resolved.Endpoint
			authToken = resolved.AuthToken
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return LsResult{}, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: authToken})
	filters := client.ListFilters{
		Name:     opts.FilterName,
		Instance: opts.FilterInstance,
		State:    opts.FilterState,
		Kind:     opts.FilterKind,
		Image:    opts.FilterImage,
	}

	var result LsResult
	if opts.IncludeNames {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting names")
		}
		names, err := cliClient.ListNames(ctx, filters)
		if err != nil {
			return result, err
		}
		if names == nil {
			names = []client.NameEntry{}
		}
		result.Names = &names
	}
	if opts.IncludeInstances {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting instances")
		}
		instances, err := cliClient.ListInstances(ctx, filters)
		if err != nil {
			return result, err
		}
		if instances == nil {
			instances = []client.InstanceEntry{}
		}
		result.Instances = &instances
	}
	if opts.IncludeStates {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting states")
		}
		states, err := cliClient.ListStates(ctx, filters)
		if err != nil {
			return result, err
		}
		if states == nil {
			states = []client.StateEntry{}
		}
		result.States = &states
	}

	return result, nil
}

func PrintLs(w io.Writer, result LsResult, opts LsPrintOptions) {
	sections := 0
	if result.Names != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Names")
		}
		printNamesTable(w, *result.Names, opts.NoHeader)
		sections++
	}
	if result.Instances != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Instances")
		}
		printInstancesTable(w, *result.Instances, opts.NoHeader)
		sections++
	}
	if result.States != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "States")
		}
		printStatesTable(w, *result.States, opts.NoHeader)
	}
}

func printNamesTable(w io.Writer, rows []client.NameEntry, noHeader bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "NAME\tINSTANCE_ID\tIMAGE_ID\tSTATE_ID\tSTATUS\tLAST_USED")
	}
	for _, row := range rows {
		stateID := row.StateID
		if stateID == "" && row.StateFingerprint != "" {
			stateID = row.StateFingerprint
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			row.Name,
			optionalString(row.InstanceID),
			row.ImageID,
			stateID,
			row.Status,
			optionalString(row.LastUsedAt),
		)
	}
	_ = tw.Flush()
}

func printInstancesTable(w io.Writer, rows []client.InstanceEntry, noHeader bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "INSTANCE_ID\tIMAGE_ID\tSTATE_ID\tNAME\tCREATED\tEXPIRES\tSTATUS")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.InstanceID,
			row.ImageID,
			row.StateID,
			optionalString(row.Name),
			row.CreatedAt,
			optionalString(row.ExpiresAt),
			row.Status,
		)
	}
	_ = tw.Flush()
}

func printStatesTable(w io.Writer, rows []client.StateEntry, noHeader bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "STATE_ID\tIMAGE_ID\tPREPARE_KIND\tPREPARE_ARGS\tCREATED\tSIZE\tREFCOUNT")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.StateID,
			row.ImageID,
			row.PrepareKind,
			row.PrepareArgs,
			row.CreatedAt,
			optionalInt64(row.SizeBytes),
			strconv.Itoa(row.RefCount),
		)
	}
	_ = tw.Flush()
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalInt64(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}
