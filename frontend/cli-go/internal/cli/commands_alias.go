package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/sqlrs/cli/internal/alias"
)

func PrintAliasEntries(w io.Writer, entries []alias.Entry) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tREF\tFILE\tKIND\tSTATUS")
	for _, entry := range entries {
		kind := entry.Kind
		if strings.TrimSpace(kind) == "" {
			kind = "-"
		}
		status := strings.ToUpper(strings.TrimSpace(entry.Status))
		if status == "" {
			status = "OK"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", entry.Class, entry.Ref, entry.File, kind, status)
	}
	_ = tw.Flush()
}

func PrintAliasCheck(w io.Writer, report alias.CheckReport) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tTYPE\tREF\tFILE\tKIND\tERROR")
	for _, result := range report.Results {
		status := "VALID"
		if !result.Valid {
			status = "INVALID"
		}
		kind := result.Kind
		if strings.TrimSpace(kind) == "" {
			kind = "-"
		}
		errorText := result.Error
		if strings.TrimSpace(errorText) == "" {
			errorText = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", status, result.Type, result.Ref, result.File, kind, errorText)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "checked=%d valid=%d invalid=%d\n", report.Checked, report.ValidCount, report.InvalidCount)
}

func PrintAliasCreate(w io.Writer, result alias.CreateResult) {
	fmt.Fprintf(w, "created alias file: %s\n", result.File)
	fmt.Fprintf(w, "type: %s\n", result.Type)
	fmt.Fprintf(w, "ref: %s\n", result.Ref)
	fmt.Fprintf(w, "kind: %s\n", result.Kind)
	if strings.TrimSpace(result.Image) != "" {
		fmt.Fprintf(w, "image: %s\n", result.Image)
	}
}
