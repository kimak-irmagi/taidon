package discover

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
)

const (
	// AnalyzerAliases keeps the existing alias discovery analyzer in the stable
	// discover registry defined by docs/architecture/discover-component-structure.md.
	AnalyzerAliases        = "aliases"
	AnalyzerGitignore      = "gitignore"
	AnalyzerVSCode         = "vscode"
	AnalyzerPrepareShaping = "prepare-shaping"

	ShellFamilyPOSIX      = "posix"
	ShellFamilyPowerShell = "powershell"
)

var stableAnalyzers = []string{
	AnalyzerAliases,
	AnalyzerGitignore,
	AnalyzerVSCode,
	AnalyzerPrepareShaping,
}

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

type analyzerRunner func(Options) (Report, error)

var analyzerRegistry = map[string]analyzerRunner{
	AnalyzerAliases:        AnalyzeAliases,
	AnalyzerGitignore:      AnalyzeGitignore,
	AnalyzerVSCode:         AnalyzeVSCode,
	AnalyzerPrepareShaping: AnalyzePrepareShaping,
}

// NormalizeSelectedAnalyzers applies the discover analyzer selection rules:
// deduplicate, keep only known analyzers, and preserve canonical order.
func NormalizeSelectedAnalyzers(selected []string) ([]string, error) {
	if len(selected) == 0 {
		return slices.Clone(stableAnalyzers), nil
	}

	seen := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		name := strings.ToLower(strings.TrimSpace(value))
		if name == "" {
			continue
		}
		if _, ok := analyzerRegistry[name]; !ok {
			return nil, fmt.Errorf("unknown discover option: --%s", name)
		}
		seen[name] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for _, name := range stableAnalyzers {
		if _, ok := seen[name]; ok {
			result = append(result, name)
		}
	}
	return result, nil
}

func defaultShellFamily() string {
	if runtime.GOOS == "windows" {
		return ShellFamilyPowerShell
	}
	return ShellFamilyPOSIX
}

func normalizedShellFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ShellFamilyPowerShell:
		return ShellFamilyPowerShell
	case ShellFamilyPOSIX:
		return ShellFamilyPOSIX
	default:
		return defaultShellFamily()
	}
}

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
