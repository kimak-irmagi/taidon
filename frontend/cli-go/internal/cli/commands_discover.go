package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sqlrs/cli/internal/discover"
)

func PrintDiscover(w io.Writer, report discover.Report) {
	fmt.Fprintf(w, "scanned=%d prefiltered=%d validated=%d suppressed=%d findings=%d\n",
		report.Scanned, report.Prefiltered, report.Validated, report.Suppressed, len(report.Findings))
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "no advisory alias candidates found")
		return
	}

	for i, finding := range report.Findings {
		if i > 0 {
			fmt.Fprintln(w)
		}
		status := "VALID"
		detailLabel := "Reason"
		detail := strings.TrimSpace(finding.Reason)
		if !finding.Valid {
			status = "INVALID"
			detailLabel = "Error"
			if errorText := strings.TrimSpace(finding.Error); errorText != "" {
				detail = errorText
			}
		}
		if detail == "" {
			detail = "-"
		}
		createCommand := strings.TrimSpace(finding.CreateCommand)
		if createCommand == "" {
			createCommand = "-"
		}

		fmt.Fprintf(w, "%d. %s %s\n", i+1, status, strings.TrimSpace(string(finding.Type)))
		printDiscoverField(w, "Ref", finding.Ref)
		printDiscoverField(w, "Kind", finding.Kind)
		printDiscoverField(w, "File", finding.File)
		printDiscoverField(w, "Alias path", finding.AliasPath)
		printDiscoverField(w, "Score", strconv.Itoa(finding.Score))
		printDiscoverField(w, detailLabel, detail)
		printDiscoverField(w, "Create command", createCommand)
	}
}

func printDiscoverField(w io.Writer, label string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "-"
	}
	fmt.Fprintf(w, "   %-14s: %s\n", label, trimmed)
}
