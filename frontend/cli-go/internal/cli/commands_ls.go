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
	Quiet         bool
	NoHeader      bool
	LongIDs       bool
	Wide          bool
	ShowSignature bool
	CacheDetails  bool
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
		printJobsTable(w, *result.Jobs, opts.NoHeader, opts.LongIDs, opts.Wide, opts.ShowSignature)
		sections++
	}
	if result.Tasks != nil {
		if sections > 0 {
			fmt.Fprintln(w)
		}
		if !opts.Quiet {
			fmt.Fprintln(w, "Tasks")
		}
		printTasksTable(w, *result.Tasks, opts.NoHeader, opts.LongIDs, opts.Wide)
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

type stateDisplayRow struct {
	stateID           string
	imageID           string
	kind              string
	prepareArgs       string
	createdAt         string
	size              string
	refCount          string
	lastUsed          string
	useCount          string
	minRetentionUntil string
}

var lsNow = func() time.Time {
	return time.Now().UTC()
}

func printStatesTable(w io.Writer, rows []client.StateEntry, noHeader bool, longIDs bool) {
	printStatesTableWithOptions(w, rows, noHeader, longIDs, false, false)
}

func printStatesTableWithOptions(w io.Writer, rows []client.StateEntry, noHeader bool, longIDs bool, wide bool, cacheDetails bool) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if !noHeader {
		if cacheDetails {
			fmt.Fprintln(tw, "STATE_ID	IMAGE_ID	KIND	PREPARE_ARGS	CREATED	SIZE	REFCOUNT	LAST_USED	USE_COUNT	MIN_RETENTION_UNTIL")
		} else {
			fmt.Fprintln(tw, "STATE_ID	IMAGE_ID	KIND	PREPARE_ARGS	CREATED	SIZE	REFCOUNT")
		}
	}

	type stateNode struct {
		visitKey string
		stateKey string
		row      client.StateEntry
		parent   *stateNode
		children []*stateNode
	}

	nodes := make([]*stateNode, 0, len(rows))
	byID := make(map[string]*stateNode, len(rows))
	for i, row := range rows {
		stateKey := strings.ToLower(strings.TrimSpace(row.StateID))
		if stateKey == "" {
			continue
		}
		node := &stateNode{
			visitKey: fmt.Sprintf("%s#%d", stateKey, i),
			stateKey: stateKey,
			row:      row,
		}
		nodes = append(nodes, node)
		if _, exists := byID[stateKey]; !exists {
			byID[stateKey] = node
		}
	}

	for _, node := range nodes {
		if node.row.ParentStateID == nil {
			continue
		}
		parentKey := strings.ToLower(strings.TrimSpace(*node.row.ParentStateID))
		if parentKey == "" || parentKey == node.stateKey {
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
		if node == nil || visited[node.visitKey] {
			return
		}
		visited[node.visitKey] = true

		stateID := formatID(node.row.StateID, longIDs)
		if depth > 0 {
			stateID = compactTreePrefix(ancestorsHasNext, isLast) + stateID
		}
		displayRows = append(displayRows, stateDisplayRow{
			stateID:           stateID,
			imageID:           formatImageID(node.row.ImageID, longIDs),
			kind:              node.row.PrepareKind,
			prepareArgs:       strings.TrimSpace(node.row.PrepareArgs),
			createdAt:         formatStateCreated(node.row.CreatedAt, longIDs),
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
		if !visited[node.visitKey] {
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
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				row.stateID,
				row.imageID,
				row.kind,
				prepareArgs,
				row.createdAt,
				row.size,
				row.refCount,
				row.lastUsed,
				row.useCount,
				row.minRetentionUntil,
			)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				row.stateID,
				row.imageID,
				row.kind,
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
	nonTTYWideColumnWidth     = 96
	ttyWideColumnSafetyMargin = 1
	stateTableColumnPadding   = 2
	stateTableDefaultGapCount = 6
	stateTableCacheGapCount   = 9
	statePrepareArgsEllipsis  = " ... "
)

func statePrepareArgsBudget(w io.Writer, rows []stateDisplayRow, noHeader bool, cacheDetails bool) int {
	budget := nonTTYWideColumnWidth
	width, ok := terminalWidth(w)
	if !ok {
		return budget
	}
	remaining := width - stateTableFixedColumnsWidth(rows, noHeader, cacheDetails) - ttyWideColumnSafetyMargin
	return clampStatePrepareArgsWidth(remaining)
}

func stateTableFixedColumnsWidth(rows []stateDisplayRow, noHeader bool, cacheDetails bool) int {
	widths := []int{0, 0, 0, 0, 0, 0}
	for _, row := range rows {
		widths[0] = maxInt(widths[0], runeLen(row.stateID))
		widths[1] = maxInt(widths[1], runeLen(row.imageID))
		widths[2] = maxInt(widths[2], runeLen(row.kind))
		widths[3] = maxInt(widths[3], runeLen(row.createdAt))
		widths[4] = maxInt(widths[4], runeLen(row.size))
		widths[5] = maxInt(widths[5], runeLen(row.refCount))
	}
	if !noHeader {
		widths[0] = maxInt(widths[0], len("STATE_ID"))
		widths[1] = maxInt(widths[1], len("IMAGE_ID"))
		widths[2] = maxInt(widths[2], len("KIND"))
		widths[3] = maxInt(widths[3], len("CREATED"))
		widths[4] = maxInt(widths[4], len("SIZE"))
		widths[5] = maxInt(widths[5], len("REFCOUNT"))
	}
	if !cacheDetails {
		return sumInts(widths) + stateTableDefaultGapCount*stateTableColumnPadding
	}

	widths = append(widths, 0, 0, 0)
	for _, row := range rows {
		widths[6] = maxInt(widths[6], runeLen(row.lastUsed))
		widths[7] = maxInt(widths[7], runeLen(row.useCount))
		widths[8] = maxInt(widths[8], runeLen(row.minRetentionUntil))
	}
	if !noHeader {
		headers := []string{"LAST_USED", "USE_COUNT", "MIN_RETENTION_UNTIL"}
		for i, header := range headers {
			widths[6+i] = maxInt(widths[6+i], len(header))
		}
	}
	return sumInts(widths) + stateTableCacheGapCount*stateTableColumnPadding
}

func formatStateCreated(value string, long bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	ts = ts.UTC()
	if long {
		return ts.Format(time.RFC3339)
	}
	return formatRelativeTime(lsNow().UTC(), ts)
}

func formatRelativeTime(now time.Time, ts time.Time) string {
	delta := now.Sub(ts)
	future := delta < 0
	if future {
		delta = -delta
	}
	value := 0
	unit := "s"
	switch {
	case delta >= 24*time.Hour:
		value = int(delta / (24 * time.Hour))
		unit = "d"
	case delta >= time.Hour:
		value = int(delta / time.Hour)
		unit = "h"
	case delta >= time.Minute:
		value = int(delta / time.Minute)
		unit = "m"
	default:
		value = int(delta / time.Second)
		if value <= 0 {
			value = 0
		}
	}
	if future {
		return fmt.Sprintf("in %d%s", value, unit)
	}
	return fmt.Sprintf("%d%s ago", value, unit)
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

const compactTableColumnGap = 1

type jobDisplayRow struct {
	jobID       string
	status      string
	kind        string
	imageID     string
	prepareArgs string
	signature   string
	planOnly    string
	created     string
	started     string
	finished    string
}

func printJobsTable(w io.Writer, rows []client.PrepareJobEntry, noHeader bool, longIDs bool, wide bool, showSignature bool) {
	headers := []string{"JOB_ID", "STATUS", "KIND", "IMAGE_ID", "PREPARE_ARGS", "PLAN_ONLY", "CREATED", "STARTED", "FINISHED"}
	if showSignature {
		headers = append(headers, "SIGNATURE")
	}
	displayRows := make([]jobDisplayRow, 0, len(rows))
	for _, row := range rows {
		displayRows = append(displayRows, jobDisplayRow{
			jobID:       formatID(row.JobID, longIDs),
			status:      row.Status,
			kind:        row.PrepareKind,
			imageID:     formatJobImageID(row, longIDs),
			prepareArgs: strings.TrimSpace(row.PrepareArgsNormalized),
			signature:   formatID(row.Signature, longIDs),
			planOnly:    formatBool(row.PlanOnly),
			created:     formatOptionalTimestamp(row.CreatedAt, longIDs),
			started:     formatOptionalTimestamp(row.StartedAt, longIDs),
			finished:    formatOptionalTimestamp(row.FinishedAt, longIDs),
		})
	}

	rowsOut := make([][]string, 0, len(displayRows))
	for _, row := range displayRows {
		prepareArgs := row.prepareArgs
		if !wide {
			prepareArgs = truncateMiddle(prepareArgs, jobsPrepareArgsBudget(w, row, noHeader))
		}
		rowsOut = append(rowsOut, []string{
			row.jobID,
			row.status,
			row.kind,
			row.imageID,
			prepareArgs,
			row.planOnly,
			row.created,
			row.started,
			row.finished,
		})
		if showSignature {
			rowsOut[len(rowsOut)-1] = append(rowsOut[len(rowsOut)-1], row.signature)
		}
	}
	printCompactTable(w, headers, rowsOut, noHeader, compactTableColumnGap)
}

type taskDisplayRow struct {
	taskID   string
	jobID    string
	taskType string
	status   string
	input    string
	args     string
	outputID string
	cached   string
}

func printTasksTable(w io.Writer, rows []client.TaskEntry, noHeader bool, longIDs bool, wide bool) {
	headers := []string{"TASK_ID", "JOB_ID", "TYPE", "STATUS", "INPUT", "ARGS", "OUTPUT_ID", "CACHED"}
	displayRows := make([]taskDisplayRow, 0, len(rows))
	for _, row := range rows {
		displayRows = append(displayRows, taskDisplayRow{
			taskID:   formatID(row.TaskID, longIDs),
			jobID:    formatID(row.JobID, longIDs),
			taskType: row.Type,
			status:   row.Status,
			input:    formatLSTaskInput(row.Input, longIDs),
			args:     strings.TrimSpace(row.ArgsSummary),
			outputID: formatID(row.OutputStateID, longIDs),
			cached:   formatCached(row.Cached),
		})
	}

	rowsOut := make([][]string, 0, len(displayRows))
	for _, row := range displayRows {
		args := row.args
		if !wide {
			args = truncateMiddle(args, tasksArgsBudget(w, row, noHeader))
		}
		rowsOut = append(rowsOut, []string{
			row.taskID,
			row.jobID,
			row.taskType,
			row.status,
			row.input,
			args,
			row.outputID,
			row.cached,
		})
	}
	printCompactTable(w, headers, rowsOut, noHeader, compactTableColumnGap)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatOptionalTimestamp(value *string, long bool) string {
	if value == nil {
		return ""
	}
	return formatStateCreated(*value, long)
}

func formatLSTaskInput(input *client.TaskInput, longIDs bool) string {
	if input == nil {
		return "unknown"
	}
	kind := strings.TrimSpace(strings.ToLower(input.Kind))
	if !longIDs {
		switch kind {
		case "state":
			kind = "s"
		case "image":
			kind = "i"
		}
	}
	id := formatID(input.ID, longIDs)
	if strings.EqualFold(strings.TrimSpace(input.Kind), "image") {
		id = formatImageID(input.ID, longIDs)
	}
	if kind == "" {
		return id
	}
	if id == "" {
		return kind
	}
	return kind + ":" + id
}

func formatJobImageID(row client.PrepareJobEntry, longIDs bool) string {
	value := strings.TrimSpace(row.ResolvedImageID)
	if value == "" {
		value = row.ImageID
	}
	return formatImageID(value, longIDs)
}

func jobsPrepareArgsBudget(w io.Writer, row jobDisplayRow, noHeader bool) int {
	width, ok := terminalWidth(w)
	if !ok {
		return nonTTYWideColumnWidth
	}
	return clampMinWideColumnWidth(width - jobsFixedColumnsWidth([]jobDisplayRow{row}, noHeader, false) - ttyWideColumnSafetyMargin)
}

func jobsFixedColumnsWidth(rows []jobDisplayRow, noHeader bool, showSignature bool) int {
	widths := []int{0, 0, 0, 0}
	for _, row := range rows {
		widths[0] = maxInt(widths[0], runeLen(row.jobID))
		widths[1] = maxInt(widths[1], runeLen(row.status))
		widths[2] = maxInt(widths[2], runeLen(row.kind))
		widths[3] = maxInt(widths[3], runeLen(row.imageID))
	}
	if !noHeader {
		headers := []string{"JOB_ID", "STATUS", "KIND", "IMAGE_ID"}
		for i, header := range headers {
			widths[i] = maxInt(widths[i], len(header))
		}
	}
	return sumInts(widths) + len(widths)*compactTableColumnGap
}

func tasksArgsBudget(w io.Writer, row taskDisplayRow, noHeader bool) int {
	width, ok := terminalWidth(w)
	if !ok {
		return nonTTYWideColumnWidth
	}
	return clampMinWideColumnWidth(width - tasksFixedColumnsWidth([]taskDisplayRow{row}, noHeader) - ttyWideColumnSafetyMargin)
}

func tasksFixedColumnsWidth(rows []taskDisplayRow, noHeader bool) int {
	widths := []int{0, 0, 0, 0, 0}
	for _, row := range rows {
		widths[0] = maxInt(widths[0], runeLen(row.taskID))
		widths[1] = maxInt(widths[1], runeLen(row.jobID))
		widths[2] = maxInt(widths[2], runeLen(row.taskType))
		widths[3] = maxInt(widths[3], runeLen(row.status))
		widths[4] = maxInt(widths[4], runeLen(row.input))
	}
	if !noHeader {
		headers := []string{"TASK_ID", "JOB_ID", "TYPE", "STATUS", "INPUT"}
		for i, header := range headers {
			widths[i] = maxInt(widths[i], len(header))
		}
	}
	return sumInts(widths) + len(widths)*compactTableColumnGap
}

func clampMinWideColumnWidth(width int) int {
	if width < statePrepareArgsMinWidth {
		return statePrepareArgsMinWidth
	}
	return width
}

func printAlignedTable(w io.Writer, headers []string, rows [][]string, noHeader bool, gap int) {
	widths := alignedTableWidths(headers, rows, noHeader)
	if !noHeader {
		writeAlignedRow(w, headers, widths, gap)
	}
	for _, row := range rows {
		writeAlignedRow(w, row, widths, gap)
	}
}

func printCompactTable(w io.Writer, headers []string, rows [][]string, noHeader bool, gap int) {
	if !noHeader {
		writeCompactRow(w, headers, gap, false)
	}
	for _, row := range rows {
		writeCompactRow(w, row, gap, true)
	}
}

func alignedTableWidths(headers []string, rows [][]string, noHeader bool) []int {
	widths := make([]int, len(headers))
	if !noHeader {
		for i, header := range headers {
			widths[i] = runeLen(header)
		}
	}
	for _, row := range rows {
		for i := 0; i < len(row) && i < len(widths); i++ {
			widths[i] = maxInt(widths[i], runeLen(row[i]))
		}
	}
	return widths
}

func writeAlignedRow(w io.Writer, row []string, widths []int, gap int) {
	for i, cell := range row {
		if i > 0 {
			io.WriteString(w, strings.Repeat(" ", gap))
		}
		io.WriteString(w, cell)
		if i == len(row)-1 || i >= len(widths) {
			continue
		}
		padding := widths[i] - runeLen(cell)
		if padding > 0 {
			io.WriteString(w, strings.Repeat(" ", padding))
		}
	}
	io.WriteString(w, "\n")
}

func writeCompactRow(w io.Writer, row []string, gap int, skipEmpty bool) {
	wroteCell := false
	for _, cell := range row {
		if skipEmpty && cell == "" {
			continue
		}
		if wroteCell {
			io.WriteString(w, strings.Repeat(" ", gap))
		}
		io.WriteString(w, cell)
		wroteCell = true
	}
	io.WriteString(w, "\n")
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
