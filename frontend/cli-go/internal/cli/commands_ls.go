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
	LongIDs  bool
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

	stateMatch := resolvedMatch{}
	if opts.FilterState != "" && (opts.IncludeStates || opts.IncludeInstances || opts.IncludeNames) {
		match, err := resolveStatePrefix(ctx, cliClient, opts.FilterState, opts.FilterKind, opts.FilterImage)
		if err != nil {
			return LsResult{}, err
		}
		stateMatch = match
	}

	instanceMatch := resolvedMatch{}
	if opts.FilterInstance != "" && (opts.IncludeInstances || opts.IncludeNames) {
		if stateMatch.noMatch {
			instanceMatch.noMatch = true
		} else {
			match, err := resolveInstancePrefix(ctx, cliClient, opts.FilterInstance, stateMatch.value, opts.FilterImage)
			if err != nil {
				return LsResult{}, err
			}
			instanceMatch = match
		}
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
		} else if stateMatch.noMatch || instanceMatch.noMatch {
			empty := []client.NameEntry{}
			result.Names = &empty
		} else {
			filterState := opts.FilterState
			if stateMatch.value != "" {
				filterState = stateMatch.value
			}
			filterInstance := opts.FilterInstance
			if instanceMatch.value != "" {
				filterInstance = instanceMatch.value
			}
			names, err := cliClient.ListNames(ctx, client.ListFilters{
				Instance: filterInstance,
				Image:    opts.FilterImage,
				State:    filterState,
			})
			if err != nil {
				return result, err
			}
			if names == nil {
				names = []client.NameEntry{}
			}
		}
	}
	if opts.IncludeInstances {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting instances")
		}
		if opts.FilterInstance != "" {
			if instanceMatch.noMatch {
				empty := []client.InstanceEntry{}
				result.Instances = &empty
			} else if instanceMatch.value != "" {
				instances := []client.InstanceEntry{instanceMatch.instance}
				result.Instances = &instances
			} else {
				instances, err := cliClient.ListInstances(ctx, client.ListFilters{
					State:    stateMatch.valueOr(opts.FilterState),
					Image:    opts.FilterImage,
					IDPrefix: "",
				})
				if err != nil {
					return result, err
				}
				if instances == nil {
					instances = []client.InstanceEntry{}
				}
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
		} else if stateMatch.noMatch {
			empty := []client.InstanceEntry{}
			result.Instances = &empty
		} else {
			instances, err := cliClient.ListInstances(ctx, client.ListFilters{
				State: stateMatch.valueOr(opts.FilterState),
				Image: opts.FilterImage,
			})
			if err != nil {
				return result, err
			}
			if instances == nil {
				instances = []client.InstanceEntry{}
			}
		}
	}
	if opts.IncludeStates {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting states")
		}
		if opts.FilterState != "" {
			if stateMatch.noMatch {
				empty := []client.StateEntry{}
				result.States = &empty
			} else if stateMatch.value != "" {
				states := []client.StateEntry{stateMatch.state}
				result.States = &states
			} else {
				states, err := cliClient.ListStates(ctx, client.ListFilters{
					Kind:     opts.FilterKind,
					Image:    opts.FilterImage,
					IDPrefix: "",
				})
				if err != nil {
					return result, err
				}
				if states == nil {
					states = []client.StateEntry{}
				}
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
		printNamesTable(w, *result.Names, opts.NoHeader, opts.LongIDs)
		sections++
	}
	if result.Instances != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Instances")
		}
		printInstancesTable(w, *result.Instances, opts.NoHeader, opts.LongIDs)
		sections++
	}
	if result.States != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "States")
		}
		printStatesTable(w, *result.States, opts.NoHeader, opts.LongIDs)
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
			formatIDPtr(row.InstanceID, longIDs),
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

type resolvedMatch struct {
	value    string
	noMatch  bool
	state    client.StateEntry
	instance client.InstanceEntry
}

func (m resolvedMatch) valueOr(fallback string) string {
	if m.value != "" {
		return m.value
	}
	return fallback
}

func resolveStatePrefix(ctx context.Context, cliClient *client.Client, prefix, kind, image string) (resolvedMatch, error) {
	normalized, err := normalizeHexPrefix(prefix)
	if err != nil {
		return resolvedMatch{}, fmt.Errorf("invalid state id prefix: %s", err)
	}
	states, err := cliClient.ListStates(ctx, client.ListFilters{
		Kind:     kind,
		Image:    image,
		IDPrefix: normalized,
	})
	if err != nil {
		return resolvedMatch{}, err
	}
	if len(states) == 0 {
		return resolvedMatch{noMatch: true}, nil
	}
	if len(states) > 1 {
		return resolvedMatch{}, fmt.Errorf("ambiguous id prefix: %s", normalized)
	}
	return resolvedMatch{value: states[0].StateID, state: states[0]}, nil
}

func resolveInstancePrefix(ctx context.Context, cliClient *client.Client, prefix, stateID, image string) (resolvedMatch, error) {
	normalized, err := normalizeHexPrefix(prefix)
	if err != nil {
		return resolvedMatch{}, fmt.Errorf("invalid instance id prefix: %s", err)
	}
	instances, err := cliClient.ListInstances(ctx, client.ListFilters{
		State:    stateID,
		Image:    image,
		IDPrefix: normalized,
	})
	if err != nil {
		return resolvedMatch{}, err
	}
	if len(instances) == 0 {
		return resolvedMatch{noMatch: true}, nil
	}
	if len(instances) > 1 {
		return resolvedMatch{}, fmt.Errorf("ambiguous id prefix: %s", normalized)
	}
	return resolvedMatch{value: instances[0].InstanceID, instance: instances[0]}, nil
}

func normalizeHexPrefix(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 8 {
		return "", fmt.Errorf("prefix must be at least 8 characters")
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return "", fmt.Errorf("prefix must be hex")
		}
	}
	return strings.ToLower(value), nil
}

func formatID(value string, longIDs bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)
	if longIDs || len(value) <= 12 {
		return value
	}
	return value[:12]
}

func formatIDPtr(value *string, longIDs bool) string {
	if value == nil {
		return ""
	}
	return formatID(*value, longIDs)
}
