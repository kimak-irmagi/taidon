package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
