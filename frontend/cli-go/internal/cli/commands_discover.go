package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sqlrs/cli/internal/discover"
)

func PrintDiscover(w io.Writer, report discover.Report) {
	fmt.Fprintf(w, "scanned=%d prefiltered=%d validated=%d suppressed=%d findings=%d\n",
		report.Scanned, report.Prefiltered, report.Validated, report.Suppressed, len(report.Findings))
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "no advisory alias candidates found")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tTYPE\tREF\tFILE\tALIAS PATH\tKIND\tSCORE\tDETAIL\tCREATE COMMAND")
	for _, finding := range report.Findings {
		status := "VALID"
		if !finding.Valid {
			status = "INVALID"
		}
		detail := strings.TrimSpace(finding.Reason)
		if !finding.Valid && strings.TrimSpace(finding.Error) != "" {
			detail = finding.Error
		}
		if detail == "" {
			detail = "-"
		}
		createCommand := strings.TrimSpace(finding.CreateCommand)
		if createCommand == "" {
			createCommand = "-"
		}
		aliasPath := strings.TrimSpace(finding.AliasPath)
		if aliasPath == "" {
			aliasPath = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			status, finding.Type, finding.Ref, finding.File, aliasPath, finding.Kind, finding.Score, detail, createCommand)
	}
	_ = tw.Flush()
}
