package discover

// Analyze runs the selected discover analyzers, isolates analyzer-local
// failures, and merges their findings into one report as described in
// docs/architecture/discover-flow.md.
func Analyze(opts Options) (Report, error) {
	selected, err := NormalizeSelectedAnalyzers(opts.SelectedAnalyzers)
	if err != nil {
		return Report{}, err
	}

	runOpts := opts
	runOpts.SelectedAnalyzers = selected
	runOpts.ShellFamily = normalizedShellFamily(opts.ShellFamily)

	report := Report{
		SelectedAnalyzers: selected,
		Summaries:         make([]AnalyzerSummary, 0, len(selected)),
		Findings:          make([]Finding, 0),
	}
	for _, analyzer := range selected {
		runner := analyzerRegistry[analyzer]
		if runner == nil {
			continue
		}

		emitProgress(runOpts.Progress, ProgressEvent{Stage: ProgressStageAnalyzerStart, Analyzer: analyzer})
		part, runErr := runner(runOpts)
		if runErr != nil {
			part = Report{
				SelectedAnalyzers: []string{analyzer},
				Findings: []Finding{{
					Analyzer: analyzer,
					Target:   analyzer,
					Action:   "analyzer failed",
					Error:    runErr.Error(),
					Valid:    false,
				}},
			}
		}
		if len(part.SelectedAnalyzers) == 0 {
			part.SelectedAnalyzers = []string{analyzer}
		}
		report.Scanned += part.Scanned
		report.Prefiltered += part.Prefiltered
		report.Validated += part.Validated
		report.Suppressed += part.Suppressed
		report.Findings = append(report.Findings, part.Findings...)
		report.Summaries = append(report.Summaries, summarizeAnalyzerReport(analyzer, part))
		emitProgress(runOpts.Progress, ProgressEvent{
			Stage:       ProgressStageAnalyzerDone,
			Analyzer:    analyzer,
			Scanned:     part.Scanned,
			Prefiltered: part.Prefiltered,
			Validated:   part.Validated,
			Suppressed:  part.Suppressed,
			Findings:    len(part.Findings),
		})
	}
	return report, nil
}
