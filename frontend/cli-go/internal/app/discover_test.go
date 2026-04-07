package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/discover"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunDiscoverHumanOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	if err := os.WriteFile(filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n\\i child.sql\n"), 0o600); err != nil {
		t.Fatalf("write schema.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "child.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write child.sql: %v", err)
	}

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "discover"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{
		"selected_analyzers=aliases,gitignore,vscode,prepare-shaping",
		"[aliases]",
		"1. VALID prepare",
		"   Ref           : schema",
		"sqlrs alias create schema prepare:psql -- -f schema.sql",
		"schema.prep.s9s.yaml",
		"[gitignore]",
		".sqlrs/",
		"[vscode]",
		".vscode/settings.json",
		"suppressed=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected output, missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "\t") {
		t.Fatalf("expected block output without tabs, got %q", out)
	}
}

func TestRunDiscoverJSONOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	if err := os.WriteFile(filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n\\i child.sql\n"), 0o600); err != nil {
		t.Fatalf("write schema.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "child.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write child.sql: %v", err)
	}

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "--output=json", "discover", "--aliases"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	selected, ok := report["selected_analyzers"].([]any)
	if !ok || len(selected) != 1 || selected[0] != "aliases" {
		t.Fatalf("unexpected selected analyzers: %s", out)
	}
	findings, ok := report["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("unexpected output: %s", out)
	}
	finding, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected finding shape: %s", out)
	}
	if got := finding["create_command"]; got == "" {
		t.Fatalf("expected create_command in finding: %s", out)
	}
	if got := finding["analyzer"]; got != "aliases" {
		t.Fatalf("expected aliases analyzer in finding: %s", out)
	}
}

func TestRunDiscoverRejectsApplyFlag(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	err := Run([]string{"--workspace", workspace, "discover", "--apply"})
	if err == nil || !strings.Contains(err.Error(), "unknown discover option") {
		t.Fatalf("expected unknown discover option error, got %v", err)
	}
}

func TestRunDiscoverHelpOutputsUsage(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "discover", "--help"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{
		"sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]",
		"--gitignore",
		"--vscode",
		"read-only",
		"all stable analyzers",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected usage, missing %q: %q", want, out)
		}
	}
}

func TestParseDiscoverArgsBranches(t *testing.T) {
	showHelp, selected, err := parseDiscoverArgs([]string{"--help"})
	if err != nil || !showHelp || selected != nil {
		t.Fatalf("expected help branch, got showHelp=%v selected=%v err=%v", showHelp, selected, err)
	}

	showHelp, selected, err = parseDiscoverArgs([]string{"--vscode", "--aliases"})
	if err != nil || showHelp {
		t.Fatalf("expected analyzer selection, got showHelp=%v err=%v", showHelp, err)
	}
	if got := strings.Join(selected, ","); got != "vscode,aliases" {
		t.Fatalf("unexpected selected analyzers: %q", got)
	}

	if _, _, err := parseDiscoverArgs([]string{"discover.sql"}); err == nil || !strings.Contains(err.Error(), "does not accept arguments") {
		t.Fatalf("expected positional arg error, got %v", err)
	}
	if _, _, err := parseDiscoverArgs([]string{"--gitignore", "--prepare-shaping"}); err != nil {
		t.Fatalf("expected gitignore and prepare-shaping flags, got %v", err)
	}
	if _, _, err := parseDiscoverArgs([]string{"\u2013aliases"}); err == nil || !strings.Contains(err.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash error, got %v", err)
	}
}

func TestRunDiscoverReturnsAnalyzerError(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)

	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		return discover.Report{}, errors.New("discover failed")
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	err := Run([]string{"--workspace", workspace, "discover", "--aliases"})
	if err == nil || !strings.Contains(err.Error(), "discover failed") {
		t.Fatalf("expected analyzer error, got %v", err)
	}
}

func TestRunDiscoverJSONWriteError(t *testing.T) {
	prevAnalyze := analyzeDiscoverFn
	analyzeDiscoverFn = func(opts discover.Options) (discover.Report, error) {
		return discover.Report{SelectedAnalyzers: []string{discover.AnalyzerAliases}}, nil
	}
	t.Cleanup(func() { analyzeDiscoverFn = prevAnalyze })

	err := runDiscover(failingWriter{}, os.Stderr, commandContext{
		workspaceRoot: t.TempDir(),
		cwd:           t.TempDir(),
		verbose:       true,
	}, []string{"--aliases"}, "json")
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected JSON write error, got %v", err)
	}
}
