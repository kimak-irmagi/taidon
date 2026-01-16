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
	Long     bool
}

type LsResult struct {
	Names     *[]client.NameEntry     `json:"names,omitempty"`
	Instances *[]client.InstanceEntry `json:"instances,omitempty"`
	States    *[]client.StateEntry    `json:"states,omitempty"`
}

type LsPrintOptions struct {
	Quiet    bool
	NoHeader bool
	Long     bool
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

	var resolvedInstance *client.InstanceEntry
	var resolvedState *client.StateEntry
	var instanceFound bool
	var stateFound bool
	if opts.FilterInstance != "" {
		entry, found, err := resolveInstancePrefix(ctx, cliClient, opts.FilterInstance)
		if err != nil {
			return LsResult{}, err
		}
		resolvedInstance = entry
		instanceFound = found
	}
	if opts.FilterState != "" {
		entry, found, err := resolveStatePrefix(ctx, cliClient, opts.FilterState)
		if err != nil {
			return LsResult{}, err
		}
		resolvedState = entry
		stateFound = found
	}

	var result LsResult
	if opts.IncludeNames {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting names")
		}
		if opts.FilterName != "" {
			entry, found, err := cliClient.GetName(ctx, opts.FilterName)
			if err != nil {
				return result, err
			}
			if found {
				names := []client.NameEntry{entry}
				result.Names = &names
			} else {
				empty := []client.NameEntry{}
				result.Names = &empty
			}
		} else {
			if (opts.FilterInstance != "" && !instanceFound) || (opts.FilterState != "" && !stateFound) {
				empty := []client.NameEntry{}
				result.Names = &empty
			} else {
				instanceID := ""
				if resolvedInstance != nil {
					instanceID = resolvedInstance.InstanceID
				}
				stateID := ""
				if resolvedState != nil {
					stateID = resolvedState.StateID
				}
				names, err := cliClient.ListNames(ctx, client.ListFilters{
					Instance: instanceID,
					Image:    opts.FilterImage,
					State:    stateID,
				})
				if err != nil {
					return result, err
				}
				if names == nil {
					names = []client.NameEntry{}
				}
				result.Names = &names
			}
		}
	}
	if opts.IncludeInstances {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting instances")
		}
		if opts.FilterInstance != "" {
			if !instanceFound {
				empty := []client.InstanceEntry{}
				result.Instances = &empty
			} else {
				instances := []client.InstanceEntry{*resolvedInstance}
				result.Instances = &instances
			}
		} else if opts.FilterName != "" {
			entry, found, err := cliClient.GetInstance(ctx, opts.FilterName)
			if err != nil {
				return result, err
			}
			if found {
				instances := []client.InstanceEntry{entry}
				result.Instances = &instances
			} else {
				empty := []client.InstanceEntry{}
				result.Instances = &empty
			}
		} else {
			if opts.FilterState != "" && !stateFound {
				empty := []client.InstanceEntry{}
				result.Instances = &empty
			} else {
				stateID := ""
				if resolvedState != nil {
					stateID = resolvedState.StateID
				}
				instances, err := cliClient.ListInstances(ctx, client.ListFilters{
					State: stateID,
					Image: opts.FilterImage,
				})
				if err != nil {
					return result, err
				}
				if instances == nil {
					instances = []client.InstanceEntry{}
				}
				result.Instances = &instances
			}
		}
	}
	if opts.IncludeStates {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting states")
		}
		if opts.FilterState != "" {
			if !stateFound {
				empty := []client.StateEntry{}
				result.States = &empty
			} else {
				states := []client.StateEntry{*resolvedState}
				result.States = &states
			}
		} else {
			states, err := cliClient.ListStates(ctx, client.ListFilters{
				Kind:  opts.FilterKind,
				Image: opts.FilterImage,
			})
			if err != nil {
				return result, err
			}
			if states == nil {
				states = []client.StateEntry{}
			}
			result.States = &states
		}
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
		printNamesTable(w, *result.Names, opts.NoHeader, opts.Long)
		sections++
	}
	if result.Instances != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Instances")
		}
		printInstancesTable(w, *result.Instances, opts.NoHeader, opts.Long)
		sections++
	}
	if result.States != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "States")
		}
		printStatesTable(w, *result.States, opts.NoHeader, opts.Long)
	}
}

func printNamesTable(w io.Writer, rows []client.NameEntry, noHeader bool, longIDs bool) {
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
			formatOptionalID(row.InstanceID, longIDs),
			row.ImageID,
			formatID(stateID, longIDs),
			row.Status,
			optionalString(row.LastUsedAt),
		)
	}
	_ = tw.Flush()
}

func printInstancesTable(w io.Writer, rows []client.InstanceEntry, noHeader bool, longIDs bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "INSTANCE_ID\tIMAGE_ID\tSTATE_ID\tNAME\tCREATED\tEXPIRES\tSTATUS")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			formatID(row.InstanceID, longIDs),
			row.ImageID,
			formatID(row.StateID, longIDs),
			optionalString(row.Name),
			row.CreatedAt,
			optionalString(row.ExpiresAt),
			row.Status,
		)
	}
	_ = tw.Flush()
}

func printStatesTable(w io.Writer, rows []client.StateEntry, noHeader bool, longIDs bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "STATE_ID\tIMAGE_ID\tPREPARE_KIND\tPREPARE_ARGS\tCREATED\tSIZE\tREFCOUNT")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			formatID(row.StateID, longIDs),
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

func formatOptionalID(value *string, longIDs bool) string {
	if value == nil {
		return ""
	}
	return formatID(*value, longIDs)
}

func formatID(value string, longIDs bool) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if longIDs || len(value) <= 12 {
		return value
	}
	return value[:12]
}

func resolveInstancePrefix(ctx context.Context, cliClient *client.Client, prefix string) (*client.InstanceEntry, bool, error) {
	normalized, err := normalizeIDPrefix("instance", prefix)
	if err != nil {
		return nil, false, err
	}
	entries, err := cliClient.ListInstances(ctx, client.ListFilters{IDPrefix: normalized})
	if err != nil {
		return nil, false, err
	}
	if len(entries) > 1 {
		return nil, false, &AmbiguousPrefixError{Kind: "instance", Prefix: prefix}
	}
	if len(entries) == 0 {
		return nil, false, nil
	}
	return &entries[0], true, nil
}

func resolveStatePrefix(ctx context.Context, cliClient *client.Client, prefix string) (*client.StateEntry, bool, error) {
	normalized, err := normalizeIDPrefix("state", prefix)
	if err != nil {
		return nil, false, err
	}
	entries, err := cliClient.ListStates(ctx, client.ListFilters{IDPrefix: normalized})
	if err != nil {
		return nil, false, err
	}
	if len(entries) > 1 {
		return nil, false, &AmbiguousPrefixError{Kind: "state", Prefix: prefix}
	}
	if len(entries) == 0 {
		return nil, false, nil
	}
	return &entries[0], true, nil
}
