package psql

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
)

func TestNormalizeArgsHandlesFileForms(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	args, stdinValue, err := NormalizeArgs([]string{"-f", "a.sql", "--file=b.sql", "-fc.sql", "-c", "select 1"}, resolver, strings.NewReader("stdin"))
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	if stdinValue != nil {
		t.Fatalf("expected no stdin, got %+v", stdinValue)
	}

	want := []string{
		"-f", filepath.Join(root, "a.sql"),
		"--file=" + filepath.Join(root, "b.sql"),
		"-f" + filepath.Join(root, "c.sql"),
		"-c", "select 1",
	}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %q, want %q", strings.Join(args, "|"), strings.Join(want, "|"))
	}
}

func TestNormalizeArgsReadsStdinOnceWhenReferenced(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	args, stdinValue, err := NormalizeArgs([]string{"-f", "-", "--file=query.sql"}, resolver, strings.NewReader("stdin data"))
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	if got := strings.Join(args, "|"); got != "-f|-|--file="+filepath.Join(root, "query.sql") {
		t.Fatalf("args = %q", got)
	}
	if stdinValue == nil || *stdinValue != "stdin data" {
		t.Fatalf("stdin = %+v", stdinValue)
	}
}

func TestBuildRunStepsMatchesSharedPsqlSemantics(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "query.sql"), "select 2;")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	steps, err := BuildRunSteps([]string{"-v", "ON_ERROR_STOP=1", "-c", "select 1", "--file=query.sql"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("BuildRunSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if got := strings.Join(steps[0].Args, " "); got != "-v ON_ERROR_STOP=1 -c select 1" {
		t.Fatalf("step[0] = %q", got)
	}
	if got := strings.Join(steps[1].Args, " "); got != "-v ON_ERROR_STOP=1 -f -" {
		t.Fatalf("step[1] = %q", got)
	}
	if steps[1].Stdin == nil || *steps[1].Stdin != "select 2;" {
		t.Fatalf("unexpected stdin: %+v", steps[1].Stdin)
	}
}

func TestCollectSupportsAllPsqlFileForms(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.sql"), "\\i b.sql\nselect 1;\n")
	writeFile(t, filepath.Join(root, "b.sql"), "select 2;\n")
	writeFile(t, filepath.Join(root, "c.sql"), "select 3;\n")
	resolver := inputset.NewDiffResolver(root)

	set, err := Collect([]string{"--", "--file=a.sql", "-fc.sql"}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(set.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(set.Entries))
	}
	if set.Entries[0].Path != "a.sql" || set.Entries[1].Path != "b.sql" || set.Entries[2].Path != "c.sql" {
		t.Fatalf("unexpected order: %+v", set.Entries)
	}
}

func TestValidateArgsReportsAccumulatedIssues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "dir", "nested.sql"), "select 1;\n")
	resolver := inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.prep.s9s.yaml"))

	issues := ValidateArgs([]string{"--file=", "-f../../outside.sql", "--file", "dir"}, resolver, inputset.OSFileSystem{})
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %+v", issues)
	}
	if issues[0].Code != "empty_path" {
		t.Fatalf("issues[0] = %+v", issues[0])
	}
	if issues[1].Code != "path_outside_workspace" {
		t.Fatalf("issues[1] = %+v", issues[1])
	}
	if issues[2].Code != "missing_path" {
		t.Fatalf("issues[2] = %+v", issues[2])
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
