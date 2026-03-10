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

	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/daemon"
)

type LsOptions struct {
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

	IncludeNames     bool
	IncludeInstances bool
	IncludeStates    bool
	IncludeJobs      bool
	IncludeTasks     bool

	FilterName     string
	FilterInstance string
	FilterState    string
	FilterJob      string
	FilterKind     string
	FilterImage    string

	Quiet        bool
	NoHeader     bool
	Long         bool
	CacheDetails bool
}

type LsResult struct {
	Names     *[]client.NameEntry       `json:"names,omitempty"`
	Instances *[]client.InstanceEntry   `json:"instances,omitempty"`
	States    *[]client.StateEntry      `json:"states,omitempty"`
	Jobs      *[]client.PrepareJobEntry `json:"jobs,omitempty"`
	Tasks     *[]client.TaskEntry       `json:"tasks,omitempty"`
}

type LsPrintOptions struct {
	Quiet        bool
	NoHeader     bool
	LongIDs      bool
	Wide         bool
	CacheDetails bool
}

func RunLs(ctx context.Context, opts LsOptions) (LsResult, error) {
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
			resolved, err := daemon.ConnectOrStart(ctx, daemon.ConnectOptions{
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
			result.Names = &names
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
			result.Instances = &instances
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

	if opts.IncludeJobs {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting jobs")
		}
		jobs, err := cliClient.ListPrepareJobs(ctx, opts.FilterJob)
		if err != nil {
			return result, err
		}
		if jobs == nil {
			jobs = []client.PrepareJobEntry{}
		}
		result.Jobs = &jobs
	}

	if opts.IncludeTasks {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "requesting tasks")
		}
		tasks, err := cliClient.ListTasks(ctx, opts.FilterJob)
		if err != nil {
			return result, err
		}
		if tasks == nil {
			tasks = []client.TaskEntry{}
		}
		result.Tasks = &tasks
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
		printStatesTableWithOptions(w, *result.States, opts.NoHeader, opts.LongIDs, opts.Wide, opts.CacheDetails)
		sections++
	}
	if result.Jobs != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Jobs")
		}
		printJobsTable(w, *result.Jobs, opts.NoHeader, opts.LongIDs)
		sections++
	}
	if result.Tasks != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Tasks")
		}
		printTasksTable(w, *result.Tasks, opts.NoHeader, opts.LongIDs)
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
			formatImageID(row.ImageID, longIDs),
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
			formatImageID(row.ImageID, longIDs),
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
	printStatesTableWithOptions(w, rows, noHeader, longIDs, false, false)
}

func printStatesTable(w io.Writer, rows []client.StateEntry, noHeader bool, longIDs bool) {
	printStatesTableWithOptions(w, rows, noHeader, longIDs, false, false)
}

func printStatesTableWithOptions(w io.Writer, rows []client.StateEntry, noHeader bool, longIDs bool, wide bool, cacheDetails bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		if cacheDetails {
			fmt.Fprintln(tw, "STATE_ID	IMAGE_ID	PREPARE_KIND	PREPARE_ARGS	CREATED	SIZE	REFCOUNT	LAST_USED	USE_COUNT	MIN_RETENTION_UNTIL")
		} else {
			fmt.Fprintln(tw, "STATE_ID	IMAGE_ID	PREPARE_KIND	PREPARE_ARGS	CREATED	SIZE	REFCOUNT")
		}
	}

	type stateNode struct {
		key      string
		row      client.StateEntry
		parent   *stateNode
		children []*stateNode
	}
	type stateDisplayRow struct {
		stateID           string
		imageID           string
		prepareKind       string
		prepareArgs       string
		createdAt         string
		size              string
		refCount          string
		lastUsed          string
		useCount          string
		minRetentionUntil string
	}

	nodes := make([]*stateNode, 0, len(rows))
	byID := make(map[string]*stateNode, len(rows))
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.StateID))
		if key == "" {
			continue
		}
		node := &stateNode{key: key, row: row}
		nodes = append(nodes, node)
		byID[key] = node
	}

	for _, node := range nodes {
		if node.row.ParentStateID == nil {
			continue
		}
		parentKey := strings.ToLower(strings.TrimSpace(*node.row.ParentStateID))
		if parentKey == "" || parentKey == node.key {
			continue
		}
		parent, ok := byID[parentKey]
		if !ok {
			continue
		}
		node.parent = parent
		parent.children = append(parent.children, node)
	}

	roots := make([]*stateNode, 0, len(nodes))
	for _, node := range nodes {
		if node.parent == nil {
			roots = append(roots, node)
		}
	}

	displayRows := make([]stateDisplayRow, 0, len(nodes))
	visited := make(map[string]bool, len(nodes))
	var walk func(node *stateNode, ancestorsHasNext []bool, depth int, isLast bool)
	walk = func(node *stateNode, ancestorsHasNext []bool, depth int, isLast bool) {
		if node == nil || visited[node.key] {
			return
		}
		visited[node.key] = true

		stateID := formatID(node.row.StateID, longIDs)
		if depth > 0 {
			stateID = compactTreePrefix(ancestorsHasNext, isLast) + stateID
		}
		displayRows = append(displayRows, stateDisplayRow{
			stateID:           stateID,
			imageID:           formatImageID(node.row.ImageID, longIDs),
			prepareKind:       node.row.PrepareKind,
			prepareArgs:       strings.TrimSpace(node.row.PrepareArgs),
			createdAt:         node.row.CreatedAt,
			size:              optionalInt64(node.row.SizeBytes),
			refCount:          strconv.Itoa(node.row.RefCount),
			lastUsed:          optionalString(node.row.LastUsedAt),
			useCount:          optionalInt64(node.row.UseCount),
			minRetentionUntil: optionalString(node.row.MinRetentionUntil),
		})

		childAncestors := ancestorsHasNext
		if depth > 0 {
			childAncestors = compactTreeNextAncestors(ancestorsHasNext, isLast)
		}
		for i, child := range node.children {
			walk(child, childAncestors, depth+1, i == len(node.children)-1)
		}
	}

	for _, root := range roots {
		walk(root, nil, 0, true)
	}
	for _, node := range nodes {
		if !visited[node.key] {
			walk(node, nil, 0, true)
		}
	}

	prepareBudget := statePrepareArgsMaxWidth
	if !wide {
		prepareBudget = statePrepareArgsBudget(w, displayRows, noHeader, cacheDetails)
	}

	for _, row := range displayRows {
		prepareArgs := row.prepareArgs
		if !wide {
			prepareArgs = truncateMiddle(prepareArgs, prepareBudget)
		}
		if cacheDetails {
			fmt.Fprintf(tw, "%s	%s	%s	%s	%s	%s	%s	%s	%s	%s
",
				row.stateID,
				row.imageID,
				row.prepareKind,
				prepareArgs,
				row.createdAt,
				row.size,
				row.refCount,
				row.lastUsed,
				row.useCount,
				row.minRetentionUntil,
			)
		} else {
			fmt.Fprintf(tw, "%s	%s	%s	%s	%s	%s	%s
",
				row.stateID,
				row.imageID,
				row.prepareKind,
				prepareArgs,
				row.createdAt,
				row.size,
				row.refCount,
			)
		}
	}

	_ = tw.Flush()
}

const (
	statePrepareArgsMinWidth  = 16
	statePrepareArgsMaxWidth  = 48
	stateTableColumnPadding   = 2
	stateTableDefaultGapCount = 6
	stateTableCacheGapCount   = 9
	statePrepareArgsEllipsis  = " ... "
)

func statePrepareArgsBudget(w io.Writer, rows []stateDisplayRow, noHeader bool, cacheDetails bool) int {
	budget := statePrepareArgsMaxWidth
	width, ok := terminalWidth(w)
	if !ok {
		return budget
	}
	remaining := width - stateTableFixedColumnsWidth(rows, noHeader, cacheDetails)
	return clampStatePrepareArgsWidth(remaining)
}

func stateTableFixedColumnsWidth(rows []stateDisplayRow, noHeader bool, cacheDetails bool) int {
	widths := []int{0, 0, 0, 0, 0, 0}
	if !cacheDetails {
		for _, row := range rows {
			widths[0] = maxInt(widths[0], runeLen(row.stateID))
			widths[1] = maxInt(widths[1], runeLen(row.imageID))
			widths[2] = maxInt(widths[2], runeLen(row.prepareKind))
			widths[3] = maxInt(widths[3], runeLen(row.createdAt))
			widths[4] = maxInt(widths[4], runeLen(row.size))
			widths[5] = maxInt(widths[5], runeLen(row.refCount))
		}
		if !noHeader {
			widths[0] = maxInt(widths[0], len("STATE_ID"))
			widths[1] = maxInt(widths[1], len("IMAGE_ID"))
			widths[2] = maxInt(widths[2], len("PREPARE_KIND"))
			widths[3] = maxInt(widths[3], len("CREATED"))
			widths[4] = maxInt(widths[4], len("SIZE"))
			widths[5] = maxInt(widths[5], len("REFCOUNT"))
		}
		return sumInts(widths) + stateTableDefaultGapCount*stateTableColumnPadding
	}

	widths = append(widths, 0, 0, 0)
	for _, row := range rows {
		widths[0] = maxInt(widths[0], runeLen(row.stateID))
		widths[1] = maxInt(widths[1], runeLen(row.imageID))
		widths[2] = maxInt(widths[2], runeLen(row.prepareKind))
		widths[3] = maxInt(widths[3], runeLen(row.createdAt))
		widths[4] = maxInt(widths[4], runeLen(row.size))
		widths[5] = maxInt(widths[5], runeLen(row.refCount))
		widths[6] = maxInt(widths[6], runeLen(row.lastUsed))
		widths[7] = maxInt(widths[7], runeLen(row.useCount))
		widths[8] = maxInt(widths[8], runeLen(row.minRetentionUntil))
	}
	if !noHeader {
		headers := []string{"STATE_ID", "IMAGE_ID", "PREPARE_KIND", "CREATED", "SIZE", "REFCOUNT", "LAST_USED", "USE_COUNT", "MIN_RETENTION_UNTIL"}
		for i, header := range headers {
			widths[i] = maxInt(widths[i], len(header))
		}
	}
	return sumInts(widths) + stateTableCacheGapCount*stateTableColumnPadding
}

func clampStatePrepareArgsWidth(width int) int {
	if width < statePrepareArgsMinWidth {
		return statePrepareArgsMinWidth
	}
	if width > statePrepareArgsMaxWidth {
		return statePrepareArgsMaxWidth
	}
	return width
}

func truncateMiddle(value string, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	ellipsisRunes := []rune(statePrepareArgsEllipsis)
	if width <= len(ellipsisRunes)+2 {
		if width > len(runes) {
			return value
		}
		return string(runes[:width])
	}
	available := width - len(ellipsisRunes)
	prefixLen := available / 2
	suffixLen := available - prefixLen
	return string(runes[:prefixLen]) + statePrepareArgsEllipsis + string(runes[len(runes)-suffixLen:])
}

func runeLen(value string) int {
	return len([]rune(value))
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func printJobsTable(w io.Writer, rows []client.PrepareJobEntry, noHeader bool, longIDs bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "JOB_ID\tSTATUS\tPREPARE_KIND\tIMAGE_ID\tPLAN_ONLY\tCREATED\tSTARTED\tFINISHED")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			formatID(row.JobID, longIDs),
			row.Status,
			row.PrepareKind,
			formatImageID(row.ImageID, longIDs),
			formatBool(row.PlanOnly),
			optionalString(row.CreatedAt),
			optionalString(row.StartedAt),
			optionalString(row.FinishedAt),
		)
	}
	_ = tw.Flush()
}

func printTasksTable(w io.Writer, rows []client.TaskEntry, noHeader bool, longIDs bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		fmt.Fprintln(tw, "TASK_ID\tJOB_ID\tTYPE\tSTATUS\tINPUT\tOUTPUT_STATE_ID\tCACHED")
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.TaskID,
			formatID(row.JobID, longIDs),
			row.Type,
			row.Status,
			formatTaskInput(row.Input),
			formatID(row.OutputStateID, longIDs),
			formatCached(row.Cached),
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

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
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
	normalized, err := normalizeIDPrefix("state", prefix)
	if err != nil {
		return resolvedMatch{}, err
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
		return resolvedMatch{}, &AmbiguousPrefixError{Kind: "state", Prefix: prefix}
	}
	return resolvedMatch{value: states[0].StateID, state: states[0]}, nil
}

func resolveInstancePrefix(ctx context.Context, cliClient *client.Client, prefix, stateID, image string) (resolvedMatch, error) {
	normalized, err := normalizeIDPrefix("instance", prefix)
	if err != nil {
		return resolvedMatch{}, err
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
		return resolvedMatch{}, &AmbiguousPrefixError{Kind: "instance", Prefix: prefix}
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

// formatImageID shortens digest-based image references in human-readable output.
// See docs/user-guides/sqlrs-ls.md.
func formatImageID(value string, longIDs bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if longIDs {
		return value
	}

	const digestPrefix = "sha256:"
	if strings.HasPrefix(value, digestPrefix) {
		digest := strings.TrimSpace(strings.TrimPrefix(value, digestPrefix))
		if len(digest) >= 12 && isHexString(digest[:12]) {
			return digest[:12]
		}
		return value
	}

	const atDigestMarker = "@sha256:"
	if i := strings.Index(value, atDigestMarker); i >= 0 {
		name := strings.TrimSpace(value[:i])
		digest := strings.TrimSpace(value[i+len(atDigestMarker):])
		if name != "" && len(digest) >= 12 && isHexString(digest[:12]) {
			return name + "@" + digest[:12]
		}
	}

	return value
}

func isHexString(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}
