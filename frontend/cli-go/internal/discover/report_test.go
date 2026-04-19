package discover

import "testing"

func TestSummarizeAnalyzerReportRemainsAnalyzerNeutral(t *testing.T) {
	report := Report{
		Scanned:     7,
		Prefiltered: 3,
		Validated:   2,
		Suppressed:  1,
		Findings: []Finding{
			{
				Analyzer:         AnalyzerGitignore,
				Target:           ".gitignore",
				Action:           "append ignore entries",
				SuggestedEntries: []string{".sqlrs/"},
				Valid:            true,
			},
			{
				Analyzer:    AnalyzerGitignore,
				Target:      ".gitignore",
				Action:      "append ignore entries",
				Error:       "already covered elsewhere",
				Valid:       false,
				JSONPayload: `{"ignored":true}`,
			},
		},
	}

	got := summarizeAnalyzerReport(AnalyzerGitignore, report)
	if got.Analyzer != AnalyzerGitignore {
		t.Fatalf("Analyzer = %q, want %q", got.Analyzer, AnalyzerGitignore)
	}
	if got.Scanned != 7 || got.Prefiltered != 3 || got.Validated != 2 || got.Suppressed != 1 || got.Findings != 2 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
