package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/discover"
)

func captureRunOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
	})

	runErr := fn()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdoutData, readErr := io.ReadAll(stdoutR)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	stderrData, readErr := io.ReadAll(stderrR)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	return string(stdoutData), string(stderrData), runErr
}

func TestDiscoverProgressWriterFormatsMilestones(t *testing.T) {
	var buf bytes.Buffer
	writer := newDiscoverProgressWriter(&buf)
	writer.Update(discover.ProgressEvent{Stage: discover.ProgressStageAnalyzerStart, Analyzer: discover.AnalyzerAliases})
	writer.Update(discover.ProgressEvent{Stage: discover.ProgressStageScanStart})
	writer.Update(discover.ProgressEvent{Stage: discover.ProgressStageScanProgress, Scanned: 64})
	writer.Update(discover.ProgressEvent{
		Stage:    discover.ProgressStageCandidate,
		Analyzer: discover.AnalyzerAliases,
		Class:    alias.ClassPrepare,
		Ref:      "schema",
		Kind:     "psql",
		File:     "schema.sql",
		Score:    80,
	})
	writer.Update(discover.ProgressEvent{
		Stage:    discover.ProgressStageValidated,
		Analyzer: discover.AnalyzerAliases,
		Class:    alias.ClassPrepare,
		Ref:      "schema",
		Kind:     "psql",
		File:     "schema.sql",
		Score:    80,
		Valid:    true,
	})
	writer.Update(discover.ProgressEvent{
		Stage:    discover.ProgressStageSuppressed,
		Analyzer: discover.AnalyzerAliases,
		Class:    alias.ClassPrepare,
		Ref:      "child",
		Kind:     "psql",
		File:     "child.sql",
		Reason:   "covered by existing alias",
	})
	writer.Update(discover.ProgressEvent{
		Stage:       discover.ProgressStageSummary,
		Scanned:     65,
		Prefiltered: 1,
		Validated:   1,
		Suppressed:  1,
		Findings:    1,
	})
	writer.Update(discover.ProgressEvent{Stage: discover.ProgressStageAnalyzerDone, Analyzer: discover.AnalyzerAliases, Findings: 1})

	out := buf.String()
	for _, want := range []string{
		"discover: analyzer aliases start",
		"discover: scanning workspace",
		"discover: scanned 64 files",
		"discover: candidate analyzer=aliases class=prepare ref=schema kind=psql file=schema.sql score=80",
		"discover: validated candidate analyzer=aliases class=prepare ref=schema kind=psql file=schema.sql score=80",
		"discover: suppressed candidate analyzer=aliases class=prepare ref=child kind=psql file=child.sql (covered by existing alias)",
		"discover: summary scanned=65 prefiltered=1 validated=1 suppressed=1 findings=1",
		"discover: analyzer aliases done findings=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in %q", want, out)
		}
	}
}

func TestRunDiscoverVerboseProgressToStderr(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		if got := strings.Join(opts.SelectedAnalyzers, ","); got != "aliases,gitignore,vscode,prepare-shaping" {
			t.Fatalf("unexpected selected analyzers: %q", got)
		}
		if opts.Progress == nil {
			t.Fatalf("expected progress sink")
		}
		opts.Progress.Update(discover.ProgressEvent{Stage: discover.ProgressStageAnalyzerStart, Analyzer: discover.AnalyzerAliases})
		opts.Progress.Update(discover.ProgressEvent{Stage: discover.ProgressStageScanStart})
		opts.Progress.Update(discover.ProgressEvent{Stage: discover.ProgressStagePrefilterDone, Scanned: 3, Prefiltered: 1})
		opts.Progress.Update(discover.ProgressEvent{
			Stage:    discover.ProgressStageCandidate,
			Analyzer: discover.AnalyzerAliases,
			Class:    alias.ClassPrepare,
			Ref:      "schema",
			Kind:     "psql",
			File:     "schema.sql",
			Score:    80,
		})
		opts.Progress.Update(discover.ProgressEvent{
			Stage:    discover.ProgressStageValidated,
			Analyzer: discover.AnalyzerAliases,
			Class:    alias.ClassPrepare,
			Ref:      "schema",
			Kind:     "psql",
			File:     "schema.sql",
			Score:    80,
			Valid:    true,
		})
		opts.Progress.Update(discover.ProgressEvent{
			Stage:       discover.ProgressStageSummary,
			Scanned:     3,
			Prefiltered: 1,
			Validated:   1,
			Suppressed:  0,
			Findings:    1,
		})
		opts.Progress.Update(discover.ProgressEvent{Stage: discover.ProgressStageAnalyzerDone, Analyzer: discover.AnalyzerAliases, Findings: 1})
		return discover.Report{
			SelectedAnalyzers: []string{discover.AnalyzerAliases},
			Scanned:           3,
			Prefiltered:       1,
			Validated:         1,
			Suppressed:        0,
			Findings: []discover.Finding{{
				Analyzer:      discover.AnalyzerAliases,
				Type:          alias.ClassPrepare,
				Kind:          "psql",
				Ref:           "schema",
				File:          "schema.sql",
				AliasPath:     "schema.prep.s9s.yaml",
				Reason:        "SQL file",
				CreateCommand: "sqlrs alias create schema prepare:psql -- -f schema.sql",
				Score:         80,
				Valid:         true,
			}},
		}, nil
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	stdout, stderr, err := captureRunOutput(t, func() error {
		return Run([]string{"--workspace", workspace, "--verbose", "discover"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stdout, "[aliases]") || !strings.Contains(stdout, "1. VALID prepare") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	for _, want := range []string{
		"discover: analyzer aliases start",
		"discover: scanning workspace",
		"discover: prefiltered 1 candidates from 3 scanned files",
		"discover: candidate analyzer=aliases class=prepare ref=schema kind=psql file=schema.sql score=80",
		"discover: validated candidate analyzer=aliases class=prepare ref=schema kind=psql file=schema.sql score=80",
		"discover: summary scanned=3 prefiltered=1 validated=1 suppressed=0 findings=1",
		"discover: analyzer aliases done findings=1",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected %q in %q", want, stderr)
		}
	}
}

func TestRunDiscoverSpinnerProgressToStderr(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		time.Sleep(750 * time.Millisecond)
		return discover.Report{
			SelectedAnalyzers: []string{discover.AnalyzerAliases},
			Scanned:           1,
			Prefiltered:       1,
			Validated:         1,
			Suppressed:        0,
			Findings: []discover.Finding{{
				Analyzer:      discover.AnalyzerAliases,
				Type:          alias.ClassPrepare,
				Kind:          "psql",
				Ref:           "schema",
				File:          "schema.sql",
				AliasPath:     "schema.prep.s9s.yaml",
				Reason:        "SQL file",
				CreateCommand: "sqlrs alias create schema prepare:psql -- -f schema.sql",
				Score:         80,
				Valid:         true,
			}},
		}, nil
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	_, stderr, err := captureRunOutput(t, func() error {
		return Run([]string{"--workspace", workspace, "discover"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr, "discover: scanning workspace") {
		t.Fatalf("expected spinner output in stderr, got %q", stderr)
	}
	if strings.Contains(stderr, "\x1b[2K") {
		t.Fatalf("expected ANSI-free spinner output, got %q", stderr)
	}
}

func TestRunDiscoverSpinnerDoesNotInterleaveWithReportOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		if opts.Progress != nil {
			t.Fatalf("expected no progress sink in non-verbose human mode")
		}
		time.Sleep(750 * time.Millisecond)
		return discover.Report{
			SelectedAnalyzers: []string{discover.AnalyzerAliases},
			Scanned:           1,
			Prefiltered:       1,
			Validated:         1,
			Suppressed:        0,
			Findings: []discover.Finding{{
				Analyzer:      discover.AnalyzerAliases,
				Type:          alias.ClassPrepare,
				Kind:          "psql",
				Ref:           "schema",
				File:          "schema.sql",
				AliasPath:     "schema.prep.s9s.yaml",
				Reason:        "SQL file",
				CreateCommand: "sqlrs alias create schema prepare:psql -- -f schema.sql",
				Score:         80,
				Valid:         true,
			}},
		}, nil
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	os.Stderr = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	})

	runErr := Run([]string{"--workspace", workspace, "discover"})
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read combined output: %v", readErr)
	}
	text := string(data)
	if strings.Contains(text, "\x1b[2K") {
		t.Fatalf("expected ANSI-free spinner output, got %q", text)
	}
	spinnerIdx := strings.Index(text, "discover: scanning workspace")
	reportIdx := strings.Index(text, "1. VALID prepare")
	if spinnerIdx < 0 || reportIdx < 0 {
		t.Fatalf("expected spinner and report in combined output, got %q", text)
	}
	if spinnerIdx > reportIdx {
		t.Fatalf("expected spinner output before report output, got %q", text)
	}
}

func TestRunDiscoverJSONOutputShowsProgressOnStderr(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		if opts.Progress != nil {
			t.Fatalf("expected no progress sink in non-verbose JSON mode")
		}
		time.Sleep(750 * time.Millisecond)
		return discover.Report{
			SelectedAnalyzers: []string{discover.AnalyzerAliases},
			Scanned:           1,
			Prefiltered:       1,
			Validated:         1,
			Suppressed:        0,
			Findings: []discover.Finding{{
				Analyzer:      discover.AnalyzerAliases,
				Type:          alias.ClassPrepare,
				Kind:          "psql",
				Ref:           "schema",
				File:          "schema.sql",
				AliasPath:     "schema.prep.s9s.yaml",
				Reason:        "SQL file",
				CreateCommand: "sqlrs alias create schema prepare:psql -- -f schema.sql",
				Score:         80,
				Valid:         true,
			}},
		}, nil
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	stdout, stderr, err := captureRunOutput(t, func() error {
		return Run([]string{"--workspace", workspace, "--output=json", "discover", "--aliases"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	findings, ok := report["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stderr, "discover: scanning workspace") {
		t.Fatalf("expected spinner output in stderr, got %q", stderr)
	}
	if strings.Contains(stderr, "\x1b[2K") {
		t.Fatalf("expected ANSI-free spinner output, got %q", stderr)
	}
}
