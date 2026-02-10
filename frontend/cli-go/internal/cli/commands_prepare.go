package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
	"sqlrs/cli/internal/util"

	"golang.org/x/term"
)

var (
	isTerminal  = term.IsTerminal
	getTermSize = term.GetSize
)

type PrepareOptions struct {
	ProfileName     string
	Mode            string
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
	StartupTimeout  time.Duration
	Verbose         bool

	ImageID           string
	PsqlArgs          []string
	LiquibaseArgs     []string
	LiquibaseExec     string
	LiquibaseExecMode string
	LiquibaseEnv      map[string]string
	WorkDir           string
	Stdin             *string
	PrepareKind       string
	PlanOnly          bool
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
		PrepareKind:       prepareKind,
		ImageID:           opts.ImageID,
		PsqlArgs:          opts.PsqlArgs,
		LiquibaseArgs:     opts.LiquibaseArgs,
		LiquibaseExec:     opts.LiquibaseExec,
		LiquibaseExecMode: opts.LiquibaseExecMode,
		LiquibaseEnv:      opts.LiquibaseEnv,
		WorkDir:           opts.WorkDir,
		Stdin:             opts.Stdin,
		PlanOnly:          false,
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

	eventsURL := strings.TrimSpace(accepted.EventsURL)
	if eventsURL == "" {
		return client.PrepareJobResult{}, fmt.Errorf("prepare events url missing")
	}

	status, err := waitForPrepare(ctx, cliClient, jobID, eventsURL, os.Stderr, opts.Verbose)
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
		PrepareKind:       prepareKind,
		ImageID:           opts.ImageID,
		PsqlArgs:          opts.PsqlArgs,
		LiquibaseArgs:     opts.LiquibaseArgs,
		LiquibaseExec:     opts.LiquibaseExec,
		LiquibaseExecMode: opts.LiquibaseExecMode,
		LiquibaseEnv:      opts.LiquibaseEnv,
		WorkDir:           opts.WorkDir,
		Stdin:             opts.Stdin,
		PlanOnly:          true,
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

	eventsURL := strings.TrimSpace(accepted.EventsURL)
	if eventsURL == "" {
		return PlanResult{}, fmt.Errorf("prepare events url missing")
	}

	status, err := waitForPrepare(ctx, cliClient, jobID, eventsURL, os.Stderr, opts.Verbose)
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

func waitForPrepare(ctx context.Context, cliClient *client.Client, jobID string, eventsURL string, progress io.Writer, verbose bool) (client.PrepareJobStatus, error) {
	if strings.TrimSpace(eventsURL) == "" {
		return client.PrepareJobStatus{}, fmt.Errorf("prepare events url missing")
	}
	tracker := newPrepareProgress(progress, verbose)
	defer tracker.Close()

	resumeIndex := 0
	for {
		rangeHeader := ""
		if resumeIndex > 0 {
			rangeHeader = fmt.Sprintf("events=%d-", resumeIndex)
		}
		resp, err := cliClient.StreamPrepareEvents(ctx, eventsURL, rangeHeader)
		if err != nil {
			if ctx.Err() != nil {
				return client.PrepareJobStatus{}, ctx.Err()
			}
			return client.PrepareJobStatus{}, err
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return client.PrepareJobStatus{}, fmt.Errorf("events stream returned status %d", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return client.PrepareJobStatus{}, fmt.Errorf("events stream returned status %d", resp.StatusCode)
		}

		startIndex := 0
		if resp.StatusCode == http.StatusPartialContent {
			rangeStart, err := parseEventsContentRange(resp.Header.Get("Content-Range"))
			if err != nil {
				resp.Body.Close()
				return client.PrepareJobStatus{}, err
			}
			startIndex = rangeStart
			if rangeStart > resumeIndex {
				resumeIndex = rangeStart
			}
		}

		counter := &countingReader{reader: resp.Body}
		reader := util.NewNDJSONReader(counter)
		currentIndex := startIndex
		readErr := error(nil)
		for {
			line, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				readErr = err
				break
			}
			if len(line) == 0 {
				continue
			}
			if currentIndex < resumeIndex {
				currentIndex++
				continue
			}
			var event client.PrepareJobEvent
			if err := json.Unmarshal(line, &event); err != nil {
				resp.Body.Close()
				return client.PrepareJobStatus{}, err
			}
			tracker.Update(event)
			if event.Type == "status" {
				status, found, err := cliClient.GetPrepareJob(ctx, jobID)
				if err != nil {
					resp.Body.Close()
					return client.PrepareJobStatus{}, err
				}
				if !found {
					resp.Body.Close()
					return client.PrepareJobStatus{}, fmt.Errorf("prepare job not found: %s", jobID)
				}
				switch status.Status {
				case "succeeded":
					resp.Body.Close()
					return status, nil
				case "failed":
					resp.Body.Close()
					if status.Error != nil {
						if status.Error.Details != "" {
							return client.PrepareJobStatus{}, fmt.Errorf("%s: %s", status.Error.Message, status.Error.Details)
						}
						return client.PrepareJobStatus{}, fmt.Errorf("%s", status.Error.Message)
					}
					return client.PrepareJobStatus{}, fmt.Errorf("prepare job failed")
				}
			}
			currentIndex++
			resumeIndex = currentIndex
		}
		resp.Body.Close()
		if readErr != nil {
			if ctx.Err() != nil {
				return client.PrepareJobStatus{}, ctx.Err()
			}
			continue
		}
		if resp.StatusCode == http.StatusOK && resp.ContentLength >= 0 {
			if counter.count >= resp.ContentLength {
				return client.PrepareJobStatus{}, fmt.Errorf("prepare job events stream ended without terminal status")
			}
			continue
		}
		if ctx.Err() != nil {
			return client.PrepareJobStatus{}, ctx.Err()
		}
	}
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.count += int64(n)
	return n, err
}

func parseEventsContentRange(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("missing content range")
	}
	if !strings.HasPrefix(value, "events ") {
		return 0, fmt.Errorf("unexpected content range: %s", value)
	}
	value = strings.TrimPrefix(value, "events ")
	parts := strings.SplitN(value, "/", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid content range: %s", value)
	}
	rangePart := parts[0]
	dash := strings.Index(rangePart, "-")
	if dash == -1 {
		return 0, fmt.Errorf("invalid content range: %s", value)
	}
	start, err := strconv.Atoi(strings.TrimSpace(rangePart[:dash]))
	if err != nil {
		return 0, fmt.Errorf("invalid content range: %s", value)
	}
	return start, nil
}

type prepareProgress struct {
	writer       io.Writer
	spinner      []string
	spinnerIndex int
	lastKey      string
	lastLen      int
	lastVisible  int
	wroteLine    bool
	verbose      bool
	clearLine    bool
	minSpinner   bool
	spinnerShown bool
}

func newPrepareProgress(writer io.Writer, verbose bool) *prepareProgress {
	if writer == nil {
		writer = io.Discard
	}
	return &prepareProgress{
		writer:     writer,
		spinner:    []string{"-", "\\", "|", "/"},
		verbose:    verbose,
		clearLine:  supportsAnsiClear(writer),
		minSpinner: supportsSpinnerBackspace(writer),
	}
}

func (p *prepareProgress) Update(event client.PrepareJobEvent) {
	base := formatPrepareEvent(event)
	if p.verbose {
		if p.writer != io.Discard {
			fmt.Fprintln(p.writer, base)
			p.wroteLine = true
		}
		return
	}
	key := prepareEventKey(event)
	if key == p.lastKey && p.wroteLine {
		p.spinnerIndex = (p.spinnerIndex + 1) % len(p.spinner)
		if p.minSpinner {
			p.writeSpinner(p.spinner[p.spinnerIndex])
			return
		}
		p.writeLine(base + " " + p.spinner[p.spinnerIndex])
		return
	}
	p.lastKey = key
	p.spinnerIndex = 0
	p.spinnerShown = false
	p.writeLine(base)
}

func (p *prepareProgress) Close() {
	if p.writer == io.Discard || !p.wroteLine || p.verbose {
		return
	}
	fmt.Fprint(p.writer, "\n")
}

func (p *prepareProgress) writeLine(line string) {
	if p.writer == io.Discard {
		return
	}
	width, hasWidth := terminalWidth(p.writer)
	if hasWidth && width > 0 {
		line = truncateLine(line, width)
	}
	if p.wroteLine {
		if p.clearLine {
			fmt.Fprint(p.writer, "\r\033[2K")
		} else {
			fmt.Fprint(p.writer, "\r")
		}
	}
	fmt.Fprint(p.writer, line)
	visible := len([]rune(line))
	if !p.clearLine && p.wroteLine && p.lastVisible > visible {
		padding := p.lastVisible - visible
		if hasWidth && width > 0 && visible+padding >= width {
			padding = max(0, width-1-visible)
		}
		if padding > 0 {
			fmt.Fprint(p.writer, strings.Repeat(" ", padding))
		}
	}
	p.lastLen = len(line)
	p.lastVisible = visible
	p.wroteLine = true
}

func supportsAnsiClear(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	if !isTerminal(int(file.Fd())) {
		return false
	}
	return runtime.GOOS != "windows" || os.Getenv("WT_SESSION") != "" || os.Getenv("TERM") != ""
}

func supportsSpinnerBackspace(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	if !isTerminal(int(file.Fd())) {
		return false
	}
	return runtime.GOOS != "windows" || os.Getenv("WT_SESSION") != "" || os.Getenv("TERM") != ""
}

func terminalWidth(writer io.Writer) (int, bool) {
	file, ok := writer.(*os.File)
	if !ok {
		return 0, false
	}
	if !isTerminal(int(file.Fd())) {
		return 0, false
	}
	width, _, err := getTermSize(int(file.Fd()))
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}

func truncateLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func (p *prepareProgress) writeSpinner(symbol string) {
	if p.writer == io.Discard {
		return
	}
	if width, ok := terminalWidth(p.writer); ok && width > 0 && p.lastVisible+2 > width {
		return
	}
	if p.spinnerShown {
		fmt.Fprint(p.writer, "\b"+symbol)
		return
	}
	fmt.Fprint(p.writer, " "+symbol)
	p.spinnerShown = true
}

func prepareEventKey(event client.PrepareJobEvent) string {
	return fmt.Sprintf("%s|%s|%s|%s", event.Type, event.Status, event.TaskID, event.Message)
}

func formatPrepareEvent(event client.PrepareJobEvent) string {
	message := strings.TrimSpace(event.Message)
	suffix := ""
	if message != "" {
		suffix = " - " + message
	}
	switch event.Type {
	case "status":
		if event.Status != "" {
			return fmt.Sprintf("prepare status: %s%s", event.Status, suffix)
		}
		return "prepare status" + suffix
	case "task":
		if event.TaskID != "" && event.Status != "" {
			return fmt.Sprintf("prepare task %s: %s%s", event.TaskID, event.Status, suffix)
		}
		if event.TaskID != "" {
			return fmt.Sprintf("prepare task %s%s", event.TaskID, suffix)
		}
		if event.Status != "" {
			return fmt.Sprintf("prepare task: %s%s", event.Status, suffix)
		}
		return "prepare task" + suffix
	case "result":
		return "prepare result: ready"
	case "error":
		if event.Error != nil {
			if event.Error.Details != "" {
				return fmt.Sprintf("prepare error: %s: %s", event.Error.Message, event.Error.Details)
			}
			if event.Error.Message != "" {
				return fmt.Sprintf("prepare error: %s", event.Error.Message)
			}
		}
		return "prepare error"
	default:
		if message != "" {
			return fmt.Sprintf("prepare %s: %s", event.Type, message)
		}
		return fmt.Sprintf("prepare %s", event.Type)
	}
}
