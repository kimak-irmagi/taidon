package pgbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
)

func TestRebaseArgsSupportsFileForms(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.run.s9s.yaml"))

	args, err := RebaseArgs([]string{"-fbench.sql", "--file=extra.sql", "-T", "30"}, resolver)
	if err != nil {
		t.Fatalf("RebaseArgs: %v", err)
	}
	want := []string{
		"-f" + filepath.Join(root, "aliases", "bench.sql"),
		"--file=" + filepath.Join(root, "aliases", "extra.sql"),
		"-T", "30",
	}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %q, want %q", strings.Join(args, "|"), strings.Join(want, "|"))
	}
}

func TestMaterializeArgsHandlesWeightedFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bench.sql"), "select 1;\n")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	args, stdinValue, err := MaterializeArgs([]string{"-fbench.sql@3", "-T", "30"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("MaterializeArgs: %v", err)
	}
	if got := strings.Join(args, "|"); got != "-f/dev/stdin@3|-T|30" {
		t.Fatalf("args = %q", got)
	}
	if stdinValue == nil || *stdinValue != "select 1;\n" {
		t.Fatalf("stdin = %+v", stdinValue)
	}
}

func TestValidateArgsReportsMultipleFilesAndMissingPath(t *testing.T) {
	root := t.TempDir()
	aliasPath := filepath.Join(root, "aliases", "demo.run.s9s.yaml")
	writeFile(t, filepath.Join(filepath.Dir(aliasPath), "bench.sql"), "select 1;\n")
	resolver := inputset.NewAliasResolver(root, aliasPath)

	issues := ValidateArgs([]string{"-fbench.sql", "--file=missing.sql"}, resolver, inputset.OSFileSystem{})
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %+v", issues)
	}
	if issues[0].Code != "multiple_file_args" || issues[1].Code != "missing_path" {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestCollectReturnsDirectPgbenchFileSet(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bench.sql"), "select 1;\n")
	resolver := inputset.NewDiffResolver(root)

	set, err := Collect([]string{"-f", "bench.sql", "-T", "30"}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(set.Entries) != 1 || set.Entries[0].Path != "bench.sql" {
		t.Fatalf("entries = %+v", set.Entries)
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
