package app

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
)

func TestParseAliasArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		want            aliasCommand
		wantHelp        bool
		wantErrContains string
	}{
		{name: "missing subcommand", wantErrContains: "missing alias subcommand"},
		{name: "root help", args: []string{"--help"}, wantHelp: true},
		{name: "subcommand help", args: []string{"check", "-h"}, wantHelp: true, want: aliasCommand{Action: "check"}},
		{name: "unknown subcommand", args: []string{"show"}, wantErrContains: "unknown alias subcommand"},
		{name: "unicode dash", args: []string{"check", "—run"}, wantErrContains: "Unicode dash"},
		{name: "missing from value", args: []string{"check", "--from"}, wantErrContains: "Missing value for --from"},
		{name: "missing depth value", args: []string{"check", "--depth"}, wantErrContains: "Missing value for --depth"},
		{name: "unknown option", args: []string{"check", "--bad"}, wantErrContains: "unknown alias option"},
		{name: "duplicate ref", args: []string{"check", "one", "two"}, wantErrContains: "at most one alias ref"},
		{name: "ls rejects ref", args: []string{"ls", "one"}, wantErrContains: "alias ls does not accept an alias ref"},
		{name: "check rejects from with ref", args: []string{"check", "--from", "cwd", "one"}, wantErrContains: "does not accept --from or --depth"},
		{
			name: "valid selectors and equals flags",
			args: []string{"check", "--prepare", "--from=workspace", "--depth=children"},
			want: aliasCommand{Action: "check", Prepare: true, From: "workspace", Depth: "children"},
		},
		{
			name: "valid depth value",
			args: []string{"check", "--depth", "self"},
			want: aliasCommand{Action: "check", Depth: "self"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, showHelp, err := parseAliasArgs(tt.args)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAliasArgs: %v", err)
			}
			if showHelp != tt.wantHelp {
				t.Fatalf("showHelp = %v, want %v", showHelp, tt.wantHelp)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRunAliasCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := runAliasCommand(&buf, commandContext{}, []string{"--help"}); err != nil {
		t.Fatalf("runAliasCommand: %v", err)
	}
	if !strings.Contains(buf.String(), "sqlrs alias ls") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestRunAliasCommandMapsScanErrorToExitCodeTwo(t *testing.T) {
	err := runAliasCommand(&bytes.Buffer{}, commandContext{
		workspaceRoot: filepath.Join(t.TempDir(), "missing"),
		cwd:           filepath.Join(t.TempDir(), "missing"),
	}, []string{"ls"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRunAliasCheckErrorPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	_, err := runAliasCheck(commandContext{workspaceRoot: root, cwd: root}, alias.ScanOptions{WorkspaceRoot: root, CWD: root}, aliasCommand{})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected scan-mode exit code 2, got %v", err)
	}

	_, err = runAliasCheck(commandContext{}, alias.ScanOptions{}, aliasCommand{Action: "check", Ref: "missing"})
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected single-target exit code 2, got %v", err)
	}
}

func TestRunAliasCommandCheckMapsErrorToExitCodeTwo(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	err := runAliasCommand(&bytes.Buffer{}, commandContext{
		workspaceRoot: root,
		cwd:           root,
	}, []string{"check"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

func TestRunAliasCommandJSONWriterError(t *testing.T) {
	workspace := t.TempDir()
	writePrepareAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	err := runAliasCommand(aliasErrWriter{}, commandContext{
		workspaceRoot: workspace,
		cwd:           workspace,
		output:        "json",
	}, []string{"ls"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestRunAliasCheckSingleTargetInvalid(t *testing.T) {
	workspace := t.TempDir()
	writeRunAliasFile(t, workspace, "bad.run.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - missing.sql\n")

	report, err := runAliasCheck(commandContext{
		workspaceRoot: workspace,
		cwd:           workspace,
	}, alias.ScanOptions{}, aliasCommand{Action: "check", Run: true, Ref: "bad"})
	if err != nil {
		t.Fatalf("runAliasCheck: %v", err)
	}
	if report.InvalidCount != 1 || report.ValidCount != 0 || len(report.Results) != 1 || report.Results[0].Valid {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestSelectedAliasClassHelpers(t *testing.T) {
	if got := selectedAliasClasses(aliasCommand{}); len(got) != 0 {
		t.Fatalf("expected no explicit classes, got %+v", got)
	}
	if got := selectedAliasClasses(aliasCommand{Prepare: true, Run: true}); !reflect.DeepEqual(got, []alias.Class{alias.ClassPrepare, alias.ClassRun}) {
		t.Fatalf("unexpected classes: %+v", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{}); got != "" {
		t.Fatalf("expected empty single class, got %q", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{Prepare: true}); got != alias.ClassPrepare {
		t.Fatalf("expected prepare single class, got %q", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{Prepare: true, Run: true}); got != "" {
		t.Fatalf("expected empty single class for mixed selectors, got %q", got)
	}
}

type aliasErrWriter struct{}

func (aliasErrWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("boom")
}
