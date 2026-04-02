package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
)

func TestParseAliasArgsCreate(t *testing.T) {
	got, showHelp, err := parseAliasArgs([]string{
		"create",
		"chinook",
		"prepare:psql",
		"--image", "postgres:17",
		"--",
		"-f", "queries.sql",
	})
	if err != nil {
		t.Fatalf("parseAliasArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("did not expect help")
	}

	want := aliasCommand{
		Action:  "create",
		Ref:     "chinook",
		Wrapped: "prepare:psql",
		Args:    []string{"--image", "postgres:17", "--", "-f", "queries.sql"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseAliasArgsCreateErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing ref", args: []string{"create"}, want: "missing alias ref"},
		{name: "missing wrapped command", args: []string{"create", "chinook"}, want: "missing wrapped command"},
		{name: "unknown flag", args: []string{"create", "--bad"}, want: "unknown alias option"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseAliasArgs(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRunAliasCommandCreateWritesAliasFile(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)
	workspace := writeAliasWorkspace(t, temp, "http://example.invalid")
	withWorkingDir(t, workspace)
	if err := os.WriteFile(filepath.Join(workspace, "queries.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write queries.sql: %v", err)
	}

	out, err := captureRunStdout(t, func() error {
		return Run([]string{"--workspace", workspace, "alias", "create", "chinook", "prepare:psql", "--", "-f", "queries.sql"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "created alias file: chinook.prep.s9s.yaml") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "kind: psql") {
		t.Fatalf("unexpected output: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(workspace, "chinook.prep.s9s.yaml"))
	if err != nil {
		t.Fatalf("read alias file: %v", err)
	}
	if !strings.Contains(string(data), "- queries.sql") {
		t.Fatalf("expected rendered args to reference queries.sql, got %q", string(data))
	}
}

func TestParseAliasArgsAdditionalBranches(t *testing.T) {
	t.Run("from and depth assignments", func(t *testing.T) {
		got, showHelp, err := parseAliasArgs([]string{"check", "--from=workspace", "--depth=3"})
		if err != nil || showHelp {
			t.Fatalf("parseAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if got.Action != "check" || got.From != "workspace" || got.Depth != "3" {
			t.Fatalf("unexpected parse result: %+v", got)
		}
	})

	t.Run("from and depth values", func(t *testing.T) {
		got, showHelp, err := parseAliasArgs([]string{"check", "--from", "workspace", "--depth", "3"})
		if err != nil || showHelp {
			t.Fatalf("parseAliasArgs: err=%v showHelp=%v", err, showHelp)
		}
		if got.From != "workspace" || got.Depth != "3" {
			t.Fatalf("unexpected parse result: %+v", got)
		}
	})

	t.Run("missing from value", func(t *testing.T) {
		_, _, err := parseAliasArgs([]string{"check", "--from"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --from") {
			t.Fatalf("expected missing --from error, got %v", err)
		}
	})

	t.Run("missing depth value", func(t *testing.T) {
		_, _, err := parseAliasArgs([]string{"check", "--depth"})
		if err == nil || !strings.Contains(err.Error(), "Missing value for --depth") {
			t.Fatalf("expected missing --depth error, got %v", err)
		}
	})

	t.Run("reject from with ref", func(t *testing.T) {
		_, _, err := parseAliasArgs([]string{"check", "schema", "--from", "workspace"})
		if err == nil || !strings.Contains(err.Error(), "does not accept --from or --depth") {
			t.Fatalf("expected from/depth rejection, got %v", err)
		}
	})

	t.Run("reject ls ref", func(t *testing.T) {
		_, _, err := parseAliasArgs([]string{"ls", "schema"})
		if err == nil || !strings.Contains(err.Error(), "does not accept an alias ref") {
			t.Fatalf("expected ls ref rejection, got %v", err)
		}
	})
}

func TestParseAliasCreateWrappedCommandAdditionalBranches(t *testing.T) {
	t.Run("prepare class", func(t *testing.T) {
		class, kind, err := parseAliasCreateWrappedCommand(" PREPARE:PSQL ")
		if err != nil {
			t.Fatalf("parseAliasCreateWrappedCommand: %v", err)
		}
		if class != alias.ClassPrepare || kind != "psql" {
			t.Fatalf("unexpected parse result: class=%q kind=%q", class, kind)
		}
	})

	t.Run("run class", func(t *testing.T) {
		class, kind, err := parseAliasCreateWrappedCommand("run:PGBENCH")
		if err != nil {
			t.Fatalf("parseAliasCreateWrappedCommand: %v", err)
		}
		if class != alias.ClassRun || kind != "pgbench" {
			t.Fatalf("unexpected parse result: class=%q kind=%q", class, kind)
		}
	})

	t.Run("missing command", func(t *testing.T) {
		_, _, err := parseAliasCreateWrappedCommand(" ")
		if err == nil || !strings.Contains(err.Error(), "missing wrapped command") {
			t.Fatalf("expected missing wrapped command error, got %v", err)
		}
	})

	t.Run("missing kind", func(t *testing.T) {
		_, _, err := parseAliasCreateWrappedCommand("prepare:")
		if err == nil || !strings.Contains(err.Error(), "wrapped command kind is required") {
			t.Fatalf("expected missing kind error, got %v", err)
		}
	})

	t.Run("missing separator", func(t *testing.T) {
		_, _, err := parseAliasCreateWrappedCommand("prepare")
		if err == nil || !strings.Contains(err.Error(), "prepare:<kind> or run:<kind>") {
			t.Fatalf("expected separator error, got %v", err)
		}
	})
}

func TestRunAliasCreateCoverage(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "seed.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write seed.sql: %v", err)
	}

	var out bytes.Buffer
	cmdCtx := commandContext{workspaceRoot: workspace, cwd: workspace, output: "json"}
	cmd := aliasCommand{Ref: "demo", Wrapped: "prepare:psql", Args: []string{"-f", "seed.sql"}}
	if err := runAliasCreate(&out, cmdCtx, cmd); err != nil {
		t.Fatalf("runAliasCreate: %v", err)
	}

	var got alias.CreateResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got.Type != alias.ClassPrepare || got.Ref != "demo" || got.Kind != "psql" {
		t.Fatalf("unexpected create result: %+v", got)
	}
	if got.File != "demo.prep.s9s.yaml" {
		t.Fatalf("unexpected file: %+v", got)
	}

	if _, err := os.Stat(filepath.Join(workspace, got.File)); err != nil {
		t.Fatalf("expected created file: %v", err)
	}

	err := runAliasCreate(&bytes.Buffer{}, commandContext{workspaceRoot: workspace, cwd: workspace}, cmd)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got %v", err)
	}

	err = runAliasCreate(&bytes.Buffer{}, commandContext{workspaceRoot: workspace, cwd: workspace}, aliasCommand{
		Ref:     "demo",
		Wrapped: "prepare",
		Args:    []string{"-f", "seed.sql"},
	})
	if err == nil || !strings.Contains(err.Error(), "wrapped command must be prepare:<kind> or run:<kind>") {
		t.Fatalf("expected wrapped command parse error, got %v", err)
	}
}

func TestRunAliasCommandAndCheckCoverage(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		var out bytes.Buffer
		if err := runAliasCommand(&out, commandContext{}, []string{"--help"}); err != nil {
			t.Fatalf("runAliasCommand: %v", err)
		}
		if !strings.Contains(out.String(), "sqlrs alias") {
			t.Fatalf("expected usage output, got %q", out.String())
		}
	})

	t.Run("scan error", func(t *testing.T) {
		_, err := runAliasCheck(commandContext{}, alias.ScanOptions{}, aliasCommand{})
		if err == nil || !strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("expected scan error, got %v", err)
		}
	})

	t.Run("resolve target error", func(t *testing.T) {
		_, err := runAliasCheck(commandContext{}, alias.ScanOptions{}, aliasCommand{Ref: "demo"})
		if err == nil || !strings.Contains(err.Error(), "workspace root is required") {
			t.Fatalf("expected resolve error, got %v", err)
		}
	})

	t.Run("invalid target", func(t *testing.T) {
		workspace := t.TempDir()
		if err := os.WriteFile(filepath.Join(workspace, "demo.prep.s9s.yaml"), []byte("kind: psql\nargs:\n  - -f\n  - missing.sql\n"), 0o600); err != nil {
			t.Fatalf("write alias file: %v", err)
		}
		report, err := runAliasCheck(
			commandContext{workspaceRoot: workspace, cwd: workspace},
			alias.ScanOptions{WorkspaceRoot: workspace, CWD: workspace},
			aliasCommand{Ref: "demo", Prepare: true},
		)
		if err != nil {
			t.Fatalf("runAliasCheck: %v", err)
		}
		if report.Checked != 1 || report.InvalidCount != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})
}

func TestSelectedSingleAliasClassCoverage(t *testing.T) {
	if got := selectedSingleAliasClass(aliasCommand{Prepare: true}); got != alias.ClassPrepare {
		t.Fatalf("expected prepare class, got %q", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{Run: true}); got != alias.ClassRun {
		t.Fatalf("expected run class, got %q", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{Prepare: true, Run: true}); got != "" {
		t.Fatalf("expected empty class, got %q", got)
	}
	if got := selectedSingleAliasClass(aliasCommand{}); got != "" {
		t.Fatalf("expected empty class, got %q", got)
	}
}
