package pgbench

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
)

type hookFS struct {
	stat     func(string) (fs.FileInfo, error)
	readFile func(string) ([]byte, error)
	readDir  func(string) ([]fs.DirEntry, error)
}

func (h hookFS) Stat(path string) (fs.FileInfo, error) {
	if h.stat != nil {
		return h.stat(path)
	}
	return os.Stat(path)
}

func (h hookFS) ReadFile(path string) ([]byte, error) {
	if h.readFile != nil {
		return h.readFile(path)
	}
	return os.ReadFile(path)
}

func (h hookFS) ReadDir(path string) ([]fs.DirEntry, error) {
	if h.readDir != nil {
		return h.readDir(path)
	}
	return os.ReadDir(path)
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestRebaseArgsAdditionalBranches(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.run.s9s.yaml"))

	args, err := RebaseArgs([]string{"-f", "-", "--file=/dev/stdin@2"}, resolver)
	if err != nil {
		t.Fatalf("RebaseArgs stdin: %v", err)
	}
	if got := strings.Join(args, "|"); got != "-f|/dev/stdin|--file=/dev/stdin@2" {
		t.Fatalf("unexpected rebased args: %q", got)
	}

	if _, err := RebaseArgs([]string{"-f"}, resolver); err == nil {
		t.Fatalf("expected missing file arg error")
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, err := RebaseArgs([]string{"-fbench.sql"}, badResolver); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got %v", err)
	}
	if _, err := RebaseArgs([]string{"--file=bench.sql"}, badResolver); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected equals resolver error, got %v", err)
	}
	if _, err := RebaseArgs([]string{"-f", "bench.sql"}, badResolver); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected positional resolver error, got %v", err)
	}
}

func TestMaterializeArgsAdditionalBranches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bench.sql"), "select 1;\n")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	args, stdinValue, err := MaterializeArgs([]string{"-c", "10"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("MaterializeArgs shared-only: %v", err)
	}
	if len(args) != 2 || stdinValue != nil {
		t.Fatalf("unexpected shared-only result: args=%+v stdin=%+v", args, stdinValue)
	}
	args, stdinValue, err = MaterializeArgs([]string{"-f", "bench.sql"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("MaterializeArgs positional file: %v", err)
	}
	if got := strings.Join(args, "|"); got != "-f|/dev/stdin" {
		t.Fatalf("unexpected positional args: %q", got)
	}
	if stdinValue == nil || *stdinValue != "select 1;\n" {
		t.Fatalf("unexpected positional stdin value: %+v", stdinValue)
	}

	if _, _, err := MaterializeArgs([]string{"--file"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing file arg error")
	}
	if _, _, err := MaterializeArgs([]string{"--file=-", "-f/dev/stdin"}, resolver, strings.NewReader("stdin"), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected duplicate file source error")
	}
	if _, _, err := MaterializeArgs([]string{"--file=bench.sql", "-f", "bench.sql"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected duplicate mixed positional file source error")
	}
	if _, _, err := MaterializeArgs([]string{"--file=bench.sql", "--file=bench.sql"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected duplicate equals file source error")
	}
	if _, _, err := MaterializeArgs([]string{"-fbench.sql", "-fbench.sql"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected duplicate short file source error")
	}
	if _, _, err := MaterializeArgs([]string{"--file=/dev/stdin@3"}, resolver, failingReader{}, inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected stdin read error")
	}

	errFS := hookFS{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	if _, _, err := MaterializeArgs([]string{"-fbench.sql"}, resolver, strings.NewReader(""), errFS); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected fs read error, got %v", err)
	}
	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, _, err := MaterializeArgs([]string{"-f", "bench.sql"}, badResolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected positional rewrite error, got %v", err)
	}
	if _, _, err := MaterializeArgs([]string{"--file=bench.sql"}, badResolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected equals rewrite error, got %v", err)
	}
	if _, _, err := MaterializeArgs([]string{"-fbench.sql"}, badResolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected short rewrite error, got %v", err)
	}
}

func TestPgbenchHelperBranches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "dir", "bench.sql"), "select 1;\n")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	if value, err := rebaseValue("-@3", resolver); err != nil || value != "/dev/stdin@3" {
		t.Fatalf("unexpected rebased stdin value: %q err=%v", value, err)
	}
	if value, source, err := rewriteFileValue("/dev/stdin@2", resolver); err != nil || value != "/dev/stdin@2" || source == nil || !source.UsesStdin {
		t.Fatalf("unexpected stdin source: value=%q source=%+v err=%v", value, source, err)
	}
	if _, _, err := rewriteFileValue(" ", resolver); err == nil {
		t.Fatalf("expected blank file value error")
	}
	if _, err := readSource(fileSource{UsesStdin: true}, failingReader{}, inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected stdin readSource error")
	}

	if issue, ok := validateLocalFileArg("/dev/stdin", resolver, inputset.OSFileSystem{}); ok {
		t.Fatalf("expected stdin to be ignored, got %+v", issue)
	}
	if issue, ok := validateLocalFileArg("dir", resolver, inputset.OSFileSystem{}); !ok || issue.Code != "expected_file" {
		t.Fatalf("expected expected_file issue, got %+v ok=%v", issue, ok)
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if issue, ok := validateLocalFileArg("bench.sql", badResolver, inputset.OSFileSystem{}); !ok || issue.Code != "invalid_path" {
		t.Fatalf("expected invalid_path issue, got %+v ok=%v", issue, ok)
	}

	if issues := ValidateArgs([]string{"-f"}, resolver, inputset.OSFileSystem{}); len(issues) != 1 || issues[0].Code != "missing_file_arg" {
		t.Fatalf("unexpected missing-file issues: %+v", issues)
	}
	if issues := ValidateArgs([]string{"-f", "dir/bench.sql"}, resolver, inputset.OSFileSystem{}); len(issues) != 0 {
		t.Fatalf("unexpected positional success issues: %+v", issues)
	}
	if issues := ValidateArgs([]string{"--file="}, resolver, inputset.OSFileSystem{}); len(issues) != 1 || issues[0].Code != "empty_path" {
		t.Fatalf("unexpected empty-path issues: %+v", issues)
	}
	if issues := ValidateArgs([]string{"--file=dir/bench.sql", "-f", "dir/bench.sql"}, resolver, inputset.OSFileSystem{}); len(issues) < 1 {
		t.Fatalf("expected duplicate mixed issues, got %+v", issues)
	} else {
		found := false
		for _, issue := range issues {
			if issue.Code == "multiple_file_args" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected duplicate mixed issues: %+v", issues)
		}
	}
	if issues := ValidateArgs([]string{"--file=bench.sql", "--file=bench.sql"}, resolver, inputset.OSFileSystem{}); len(issues) < 1 || issues[0].Code != "multiple_file_args" {
		found := false
		for _, issue := range issues {
			if issue.Code == "multiple_file_args" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected duplicate equals issues: %+v", issues)
		}
	}
	if issues := ValidateArgs([]string{"-fbench.sql", "-fbench.sql"}, resolver, inputset.OSFileSystem{}); len(issues) < 1 || issues[0].Code != "multiple_file_args" {
		found := false
		for _, issue := range issues {
			if issue.Code == "multiple_file_args" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected duplicate short issues: %+v", issues)
		}
	}
	if issue, ok := validateLocalFileArg("../../outside.sql", inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.run.s9s.yaml")), inputset.OSFileSystem{}); !ok || issue.Code != "path_outside_workspace" {
		t.Fatalf("expected outside-workspace issue, got %+v ok=%v", issue, ok)
	}
}

func TestCollectAdditionalBranches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bench.sql"), "select 1;\n")
	resolver := inputset.NewDiffResolver(root)

	set, err := Collect([]string{"-fbench.sql", "--file=bench.sql", "--file=-", "--file=/dev/stdin"}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect dedup: %v", err)
	}
	if len(set.Entries) != 1 || set.Entries[0].Path != "bench.sql" {
		t.Fatalf("unexpected deduped entries: %+v", set.Entries)
	}

	if _, err := Collect([]string{"-f"}, resolver, inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing file arg error")
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, err := Collect([]string{"-fbench.sql"}, badResolver, inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got %v", err)
	}

	errFS := hookFS{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := Collect([]string{"-fbench.sql"}, resolver, errFS); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestPgbenchAdditionalHelperCoverage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bench.sql"), "select 1;\n")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	if value, err := rebaseValue("/dev/stdin", resolver); err != nil || value != "/dev/stdin" {
		t.Fatalf("unexpected stdin rebase value: %q err=%v", value, err)
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, _, err := rewriteFileValue("bench.sql", badResolver); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected rewrite resolver error, got %v", err)
	}

	fileFS := hookFS{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := readSource(fileSource{Path: filepath.Join(root, "bench.sql")}, strings.NewReader(""), fileFS); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected file readSource error, got %v", err)
	}
	if value, err := readSource(fileSource{Path: filepath.Join(root, "bench.sql")}, strings.NewReader(""), inputset.OSFileSystem{}); err != nil || value != "select 1;\n" {
		t.Fatalf("unexpected file readSource success: %q err=%v", value, err)
	}
	if value, err := readSource(fileSource{UsesStdin: true}, strings.NewReader("stdin\n"), inputset.OSFileSystem{}); err != nil || value != "stdin\n" {
		t.Fatalf("unexpected stdin readSource success: %q err=%v", value, err)
	}

	set, err := Collect([]string{"--file=", "--file=/dev/stdin"}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect blank/stdin ignore: %v", err)
	}
	if len(set.Entries) != 0 {
		t.Fatalf("expected blank/stdin refs to be ignored, got %+v", set.Entries)
	}
}
