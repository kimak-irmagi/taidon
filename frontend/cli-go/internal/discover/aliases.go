package discover

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AnalyzeAliases walks the workspace, scores likely workflow roots, validates
// supported candidates, and suppresses suggestions already covered by alias
// files on disk.
func AnalyzeAliases(opts Options) (Report, error) {
	workspaceRoot, cwd, err := normalizeDiscoverRoots(opts.WorkspaceRoot, opts.CWD)
	if err != nil {
		return Report{}, err
	}

	emitProgress(opts.Progress, ProgressEvent{Stage: ProgressStageScanStart})

	coverage, err := loadAliasCoverage(workspaceRoot)
	if err != nil {
		return Report{}, err
	}

	files, scanned, err := walkDiscoverFiles(workspaceRoot, cwd, opts.Progress)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		SelectedAnalyzers: []string{AnalyzerAliases},
		Scanned:           scanned,
	}
	proposals := scoreDiscoverFiles(files)
	report.Prefiltered = len(proposals)
	emitProgress(opts.Progress, ProgressEvent{
		Stage:       ProgressStagePrefilterDone,
		Scanned:     report.Scanned,
		Prefiltered: report.Prefiltered,
	})

	validated, err := validateDiscoverProposals(proposals, workspaceRoot, cwd, opts.Progress)
	if err != nil {
		return Report{}, err
	}
	report.Validated = len(validated)

	rankValidatedCandidates(validated)
	findings, suppressed, err := selectAliasFindings(validated, coverage, workspaceRoot, cwd, opts.ShellFamily, opts.Progress)
	if err != nil {
		return Report{}, err
	}
	sortAliasFindings(findings)
	report.Suppressed = suppressed
	report.Findings = findings
	report.Summaries = []AnalyzerSummary{summarizeAnalyzerReport(AnalyzerAliases, report)}
	emitProgress(opts.Progress, ProgressEvent{
		Stage:       ProgressStageSummary,
		Scanned:     report.Scanned,
		Prefiltered: report.Prefiltered,
		Validated:   report.Validated,
		Suppressed:  report.Suppressed,
		Findings:    len(report.Findings),
	})
	return report, nil
}

func normalizeDiscoverRoots(workspaceRoot string, cwd string) (string, string, error) {
	trimmedWorkspaceRoot := strings.TrimSpace(workspaceRoot)
	if trimmedWorkspaceRoot == "" {
		return "", "", fmt.Errorf("workspace root is required for discover")
	}
	resolvedWorkspaceRoot, err := filepath.Abs(trimmedWorkspaceRoot)
	if err != nil {
		return "", "", err
	}
	resolvedWorkspaceRoot = filepath.Clean(resolvedWorkspaceRoot)

	trimmedCwd := strings.TrimSpace(cwd)
	if trimmedCwd == "" {
		trimmedCwd = resolvedWorkspaceRoot
	}
	resolvedCwd, err := filepath.Abs(trimmedCwd)
	if err != nil {
		return "", "", err
	}
	return resolvedWorkspaceRoot, filepath.Clean(resolvedCwd), nil
}
