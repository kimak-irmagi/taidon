package discover

import "github.com/sqlrs/cli/internal/alias"

const (
	// ProgressStageAnalyzerStart marks the start of one analyzer within discover.
	ProgressStageAnalyzerStart = "analyzer-start"
	// ProgressStageAnalyzerDone marks the end of one analyzer within discover.
	ProgressStageAnalyzerDone = "analyzer-done"
	// ProgressStageScanStart marks the start of workspace scanning.
	ProgressStageScanStart = "scan-start"
	// ProgressStageScanProgress marks a periodic workspace scanning heartbeat.
	ProgressStageScanProgress = "scan-progress"
	// ProgressStagePrefilterDone marks the end of the cheap prefilter stage.
	ProgressStagePrefilterDone = "prefilter-done"
	// ProgressStageCandidate marks a candidate that is about to be validated.
	ProgressStageCandidate = "candidate"
	// ProgressStageValidated marks a candidate after kind-specific validation.
	ProgressStageValidated = "validated"
	// ProgressStageSuppressed marks a candidate removed by topology or coverage.
	ProgressStageSuppressed = "suppressed"
	// ProgressStageSummary marks the final aggregated discovery summary.
	ProgressStageSummary = "summary"
)

// Progress is an optional sink for discovery milestones described in
// docs/architecture/discover-flow.md.
type Progress interface {
	Update(ProgressEvent)
}

// ProgressEvent carries one discovery milestone to the CLI progress renderer.
type ProgressEvent struct {
	Analyzer    string      `json:"analyzer,omitempty"`
	Stage       string      `json:"stage,omitempty"`
	Scanned     int         `json:"scanned,omitempty"`
	Prefiltered int         `json:"prefiltered,omitempty"`
	Validated   int         `json:"validated,omitempty"`
	Suppressed  int         `json:"suppressed,omitempty"`
	Findings    int         `json:"findings,omitempty"`
	Class       alias.Class `json:"class,omitempty"`
	Kind        string      `json:"kind,omitempty"`
	Ref         string      `json:"ref,omitempty"`
	File        string      `json:"file,omitempty"`
	Score       int         `json:"score,omitempty"`
	Reason      string      `json:"reason,omitempty"`
	Error       string      `json:"error,omitempty"`
	Valid       bool        `json:"valid,omitempty"`
}

func emitProgress(progress Progress, event ProgressEvent) {
	if progress == nil {
		return
	}
	progress.Update(event)
}
