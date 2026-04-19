package discover

// AnalyzerSummary records one analyzer-local contribution to a merged discover
// report.
type AnalyzerSummary struct {
	Analyzer    string `json:"analyzer"`
	Scanned     int    `json:"scanned,omitempty"`
	Prefiltered int    `json:"prefiltered,omitempty"`
	Validated   int    `json:"validated,omitempty"`
	Suppressed  int    `json:"suppressed,omitempty"`
	Findings    int    `json:"findings"`
}

// FollowUpCommand carries a shell-aware copy-paste command for acting on an
// advisory discover finding without making discover itself mutating.
type FollowUpCommand struct {
	ShellFamily string `json:"shell_family,omitempty"`
	Command     string `json:"command,omitempty"`
}

func summarizeAnalyzerReport(analyzer string, report Report) AnalyzerSummary {
	return AnalyzerSummary{
		Analyzer:    analyzer,
		Scanned:     report.Scanned,
		Prefiltered: report.Prefiltered,
		Validated:   report.Validated,
		Suppressed:  report.Suppressed,
		Findings:    len(report.Findings),
	}
}
