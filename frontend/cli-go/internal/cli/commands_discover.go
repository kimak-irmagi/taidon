package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sqlrs/cli/internal/discover"
)

func PrintDiscover(w io.Writer, report discover.Report) {
	if len(report.SelectedAnalyzers) > 0 {
		fmt.Fprintf(w, "selected_analyzers=%s ", strings.Join(report.SelectedAnalyzers, ","))
	}
	fmt.Fprintf(w, "scanned=%d prefiltered=%d validated=%d suppressed=%d findings=%d\n",
		report.Scanned, report.Prefiltered, report.Validated, report.Suppressed, len(report.Findings))
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "no advisory discover findings")
		return
	}

	order := report.SelectedAnalyzers
	if len(order) == 0 {
		order = []string{
			discover.AnalyzerAliases,
			discover.AnalyzerGitignore,
			discover.AnalyzerVSCode,
			discover.AnalyzerPrepareShaping,
		}
	}

	index := 0
	for _, analyzer := range order {
		group := make([]discover.Finding, 0)
		for _, finding := range report.Findings {
			if strings.TrimSpace(finding.Analyzer) == analyzer {
				group = append(group, finding)
			}
		}
		if len(group) == 0 {
			continue
		}
		if index > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "[%s]\n", analyzer)
		for _, finding := range group {
			index++
			if analyzer == discover.AnalyzerAliases {
				printAliasDiscoverFinding(w, index, finding)
				continue
			}
			printGenericDiscoverFinding(w, index, finding)
		}
	}
}

func printAliasDiscoverFinding(w io.Writer, index int, finding discover.Finding) {
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
	if createCommand == "" && finding.FollowUpCommand != nil {
		createCommand = strings.TrimSpace(finding.FollowUpCommand.Command)
	}
	if createCommand == "" {
		createCommand = "-"
	}

	fmt.Fprintf(w, "%d. %s %s\n", index, status, strings.TrimSpace(string(finding.Type)))
	printDiscoverField(w, "Ref", finding.Ref)
	printDiscoverField(w, "Kind", finding.Kind)
	printDiscoverField(w, "File", finding.File)
	printDiscoverField(w, "Alias path", finding.AliasPath)
	printDiscoverField(w, "Score", strconv.Itoa(finding.Score))
	printDiscoverField(w, detailLabel, detail)
	printDiscoverField(w, "Create command", createCommand)
}

func printGenericDiscoverFinding(w io.Writer, index int, finding discover.Finding) {
	fmt.Fprintf(w, "%d. ADVISORY %s\n", index, strings.TrimSpace(finding.Analyzer))
	printDiscoverField(w, "Target", finding.Target)
	printDiscoverField(w, "Action", finding.Action)
	if reason := strings.TrimSpace(finding.Reason); reason != "" {
		printDiscoverField(w, "Reason", reason)
	}
	if len(finding.SuggestedEntries) > 0 {
		printDiscoverField(w, "Entries", strings.Join(finding.SuggestedEntries, ", "))
	}
	if payload := strings.TrimSpace(finding.JSONPayload); payload != "" {
		printDiscoverField(w, "Payload", payload)
	}
	command := strings.TrimSpace(finding.CreateCommand)
	if command == "" && finding.FollowUpCommand != nil {
		command = strings.TrimSpace(finding.FollowUpCommand.Command)
	}
	if command != "" {
		printDiscoverField(w, "Follow-up command", command)
	}
	if finding.FollowUpCommand != nil {
		printDiscoverField(w, "Shell", finding.FollowUpCommand.ShellFamily)
	}
	if !finding.Valid && strings.TrimSpace(finding.Error) != "" {
		printDiscoverField(w, "Error", finding.Error)
	}
}

func printDiscoverField(w io.Writer, label string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "-"
	}
	lines := strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
	fmt.Fprintf(w, "   %-14s: %s\n", label, lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "   %-14s  %s\n", "", line)
	}
}
