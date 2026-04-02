package discover

import (
	"path/filepath"
	"testing"
)

type recordingProgress struct {
	events []ProgressEvent
}

func (r *recordingProgress) Update(event ProgressEvent) {
	r.events = append(r.events, event)
}

func TestAnalyzeAliasesEmitsProgressMilestones(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n\\i child.sql\n"))
	mustWriteFile(t, filepath.Join(workspace, "child.sql"), []byte("select 1;\n"))

	progress := &recordingProgress{}
	report, err := AnalyzeAliases(Options{
		WorkspaceRoot: workspace,
		CWD:           workspace,
		Progress:      progress,
	})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected one suppressed candidate, got %+v", report)
	}
	if len(progress.events) == 0 {
		t.Fatalf("expected progress events, got none")
	}
	if got := progress.events[0].Stage; got != ProgressStageScanStart {
		t.Fatalf("expected first stage %q, got %q", ProgressStageScanStart, got)
	}

	var sawPrefilter, sawCandidate, sawValidated, sawSuppressed, sawSummary bool
	for _, event := range progress.events {
		switch event.Stage {
		case ProgressStagePrefilterDone:
			sawPrefilter = true
			if event.Scanned == 0 || event.Prefiltered == 0 {
				t.Fatalf("unexpected prefilter event: %+v", event)
			}
		case ProgressStageCandidate:
			sawCandidate = true
		case ProgressStageValidated:
			sawValidated = true
		case ProgressStageSuppressed:
			sawSuppressed = true
		case ProgressStageSummary:
			sawSummary = true
			if event.Findings != len(report.Findings) {
				t.Fatalf("unexpected summary event: %+v report=%+v", event, report)
			}
		}
	}
	if !sawPrefilter || !sawCandidate || !sawValidated || !sawSuppressed || !sawSummary {
		t.Fatalf("missing progress stages: prefilter=%v candidate=%v validated=%v suppressed=%v summary=%v events=%+v",
			sawPrefilter, sawCandidate, sawValidated, sawSuppressed, sawSummary, progress.events)
	}
}
