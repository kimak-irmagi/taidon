package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/discover"
)

// discoverProgressWriter renders verbose discovery milestones to the stderr
// stream described in docs/architecture/discover-flow.md.
type discoverProgressWriter struct {
	writer io.Writer
}

func newDiscoverProgressWriter(writer io.Writer) *discoverProgressWriter {
	if writer == nil {
		writer = io.Discard
	}
	return &discoverProgressWriter{writer: writer}
}

func (p *discoverProgressWriter) Update(event discover.ProgressEvent) {
	if p == nil || p.writer == nil || p.writer == io.Discard {
		return
	}
	line := formatDiscoverProgressLine(event)
	if line == "" {
		return
	}
	fmt.Fprintln(p.writer, line)
}

func formatDiscoverProgressLine(event discover.ProgressEvent) string {
	switch event.Stage {
	case discover.ProgressStageScanStart:
		return "discover: scanning workspace"
	case discover.ProgressStageScanProgress:
		if event.Scanned <= 0 {
			return ""
		}
		return fmt.Sprintf("discover: scanned %d files", event.Scanned)
	case discover.ProgressStagePrefilterDone:
		return fmt.Sprintf("discover: prefiltered %d candidates from %d scanned files", event.Prefiltered, event.Scanned)
	case discover.ProgressStageCandidate:
		return fmt.Sprintf("discover: candidate %s", formatDiscoverProgressSubject(event))
	case discover.ProgressStageValidated:
		if !event.Valid {
			detail := strings.TrimSpace(event.Error)
			if detail != "" {
				return fmt.Sprintf("discover: invalid candidate %s: %s", formatDiscoverProgressSubject(event), detail)
			}
			return fmt.Sprintf("discover: invalid candidate %s", formatDiscoverProgressSubject(event))
		}
		return fmt.Sprintf("discover: validated candidate %s", formatDiscoverProgressSubject(event))
	case discover.ProgressStageSuppressed:
		reason := strings.TrimSpace(event.Reason)
		if reason != "" {
			return fmt.Sprintf("discover: suppressed candidate %s (%s)", formatDiscoverProgressSubject(event), reason)
		}
		return fmt.Sprintf("discover: suppressed candidate %s", formatDiscoverProgressSubject(event))
	case discover.ProgressStageSummary:
		return fmt.Sprintf("discover: summary scanned=%d prefiltered=%d validated=%d suppressed=%d findings=%d",
			event.Scanned, event.Prefiltered, event.Validated, event.Suppressed, event.Findings)
	default:
		return ""
	}
}

func formatDiscoverProgressSubject(event discover.ProgressEvent) string {
	parts := make([]string, 0, 4)
	if class := strings.TrimSpace(string(event.Class)); class != "" {
		parts = append(parts, "class="+class)
	}
	if ref := strings.TrimSpace(event.Ref); ref != "" {
		parts = append(parts, "ref="+ref)
	}
	if kind := strings.TrimSpace(event.Kind); kind != "" {
		parts = append(parts, "kind="+kind)
	}
	if file := strings.TrimSpace(event.File); file != "" {
		parts = append(parts, "file="+file)
	}
	if score := event.Score; score > 0 {
		parts = append(parts, fmt.Sprintf("score=%d", score))
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, " ")
}

var _ discover.Progress = (*discoverProgressWriter)(nil)
