package psql

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
	"github.com/sqlrs/cli/internal/pathutil"
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

func TestNormalizeArgsErrorBranches(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	if _, _, err := NormalizeArgs([]string{"-f"}, resolver, strings.NewReader("")); err == nil {
		t.Fatalf("expected missing file arg error")
	}
	if _, _, err := NormalizeArgs([]string{"-f", "-"}, resolver, failingReader{}); err == nil {
		t.Fatalf("expected stdin read error")
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, _, err := NormalizeArgs([]string{"-f", "a.sql"}, badResolver, strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected positional resolver error, got %v", err)
	}
	if _, _, err := NormalizeArgs([]string{"--file=a.sql"}, badResolver, strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got %v", err)
	}
	if _, _, err := NormalizeArgs([]string{"-fa.sql"}, badResolver, strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected short resolver error, got %v", err)
	}

	if value, useStdin, err := normalizeFileArg("-", resolver); err != nil || !useStdin || value != "-" {
		t.Fatalf("expected stdin marker, got value=%q useStdin=%v err=%v", value, useStdin, err)
	}
}

func TestBuildRunStepsAdditionalBranches(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	steps, err := BuildRunSteps([]string{"--command=select 1", "-cselect 2"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("BuildRunSteps command forms: %v", err)
	}
	if len(steps) != 2 || strings.Join(steps[0].Args, " ") != "-c select 1" || strings.Join(steps[1].Args, " ") != "-c select 2" {
		t.Fatalf("unexpected command steps: %+v", steps)
	}

	steps, err = BuildRunSteps([]string{"-v", "ON_ERROR_STOP=1"}, resolver, strings.NewReader(""), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("BuildRunSteps shared args: %v", err)
	}
	if len(steps) != 1 || strings.Join(steps[0].Args, " ") != "-v ON_ERROR_STOP=1" {
		t.Fatalf("unexpected shared-only step: %+v", steps)
	}

	if _, err := BuildRunSteps([]string{"-c"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing command value error")
	}
	if _, err := BuildRunSteps([]string{"-f"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing file value error")
	}
	if _, err := BuildRunSteps([]string{"--file=-", "-f-"}, resolver, strings.NewReader("stdin"), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected duplicate stdin file error")
	}
	if _, err := BuildRunSteps([]string{"--file= ", "-c", "select 1"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected equals file-step error")
	}
	if _, err := BuildRunSteps([]string{"-f ", "-c", "select 1"}, resolver, strings.NewReader(""), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected short file-step error")
	}
	steps, err = BuildRunSteps([]string{"--file=-"}, resolver, strings.NewReader("stdin data"), inputset.OSFileSystem{})
	if err != nil || len(steps) != 1 || steps[0].Stdin == nil || *steps[0].Stdin != "stdin data" {
		t.Fatalf("unexpected equals stdin steps: %+v err=%v", steps, err)
	}
	if _, err := BuildRunSteps([]string{"-f", "-"}, resolver, failingReader{}, inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected stdin read error")
	}
}

func TestBuildFileStepAndCollectErrorBranches(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "query.sql"), "select 1;\n")
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	if _, _, err := buildFileStep(nil, " ", resolver, inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing file value error")
	}
	if step, useStdin, err := buildFileStep([]string{"-v", "ON_ERROR_STOP=1"}, "-", resolver, inputset.OSFileSystem{}); err != nil || !useStdin || strings.Join(step.Args, " ") != "-v ON_ERROR_STOP=1 -f -" {
		t.Fatalf("unexpected stdin step: %+v useStdin=%v err=%v", step, useStdin, err)
	}

	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if _, _, err := buildFileStep(nil, "query.sql", badResolver, inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected resolver error, got %v", err)
	}

	errFS := hookFS{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	if _, _, err := buildFileStep(nil, "query.sql", resolver, errFS); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected fs read error, got %v", err)
	}

	if _, err := Collect([]string{"-c", "select 1"}, inputset.NewDiffResolver(root), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing -f diff error")
	}
	if _, err := Collect([]string{"-f"}, inputset.NewDiffResolver(root), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected missing file arg error")
	}
}

func TestCollectRecursiveIncludeAndScannerError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.sql"), "\\i b.sql\n")
	writeFile(t, filepath.Join(root, "b.sql"), "\\i a.sql\n")

	if _, err := Collect([]string{"-f", "a.sql"}, inputset.NewDiffResolver(root), inputset.OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "recursive include") {
		t.Fatalf("expected recursive include error, got %v", err)
	}

	bigRoot := t.TempDir()
	writeFile(t, filepath.Join(bigRoot, "big.sql"), strings.Repeat("x", 70_000))
	if _, err := Collect([]string{"-f", "big.sql"}, inputset.NewDiffResolver(bigRoot), inputset.OSFileSystem{}); err == nil {
		t.Fatalf("expected scanner error for oversized line")
	}
}

func TestCollectUsesResolverBaseDirForPlainInclude(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chinook", "a.sql"), "\\i b.sql\n")
	writeFile(t, filepath.Join(root, "b.sql"), "select 1;\n")

	set, err := Collect([]string{"-f", filepath.Join("chinook", "a.sql")}, inputset.NewWorkspaceResolver(root, root, nil), inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(set.Entries) != 2 || set.Entries[0].Path != "chinook/a.sql" || set.Entries[1].Path != "b.sql" {
		t.Fatalf("unexpected collected entries: %+v", set.Entries)
	}
}

func TestPsqlHelperBranches(t *testing.T) {
	root := t.TempDir()
	aliasPath := filepath.Join(root, "aliases", "demo.prep.s9s.yaml")
	aliasDir := filepath.Dir(aliasPath)
	writeFile(t, filepath.Join(aliasDir, "dir", "child.sql"), "select 1;\n")
	writeFile(t, filepath.Join(root, "base", "inner", "nested.sql"), "select 2;\n")
	resolver := inputset.NewAliasResolver(root, aliasPath)

	paths, err := collectEntryPaths([]string{"--", "--file=dir/child.sql", "-f-"}, inputset.NewDiffResolver(root))
	if err != nil {
		t.Fatalf("collectEntryPaths: %v", err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "child.sql" {
		t.Fatalf("unexpected collected paths: %+v", paths)
	}

	if cmd, arg, ok := parseInclude(`\include "dir/child.sql"`); !ok || cmd != `\include` || arg != "dir/child.sql" {
		t.Fatalf("unexpected parseInclude result: cmd=%q arg=%q ok=%v", cmd, arg, ok)
	}
	if _, _, ok := parseInclude(`\set x 1`); ok {
		t.Fatalf("expected unsupported psql meta-command to be ignored")
	}
	parts := splitCommand(`\i "base file.sql"`)
	if len(parts) != 2 || parts[1] != "base file.sql" {
		t.Fatalf("unexpected splitCommand result: %+v", parts)
	}

	tracker := &tracker{
		root:    root,
		baseDir: filepath.Join(root, "base"),
		seen:    make(map[string]struct{}),
		stack:   make(map[string]struct{}),
		fs:      inputset.OSFileSystem{},
	}
	got := tracker.resolveInclude(`\include_relative`, filepath.ToSlash(filepath.Join("inner", "nested.sql")), filepath.Join(root, "base", "a.sql"))
	if !pathutil.SameLocalPath(got, filepath.Join(root, "base", "inner", "nested.sql")) {
		t.Fatalf("unexpected relative include path: %q", got)
	}
	if got := tracker.resolveInclude(`\i`, "plain.sql", filepath.Join(root, "base", "a.sql")); !pathutil.SameLocalPath(got, filepath.Join(root, "base", "plain.sql")) {
		t.Fatalf("unexpected plain include path: %q", got)
	}
	abs := filepath.Join(root, "dir", "child.sql")
	if got := tracker.resolveInclude(`\i`, abs, filepath.Join(root, "base", "a.sql")); !pathutil.SameLocalPath(got, abs) {
		t.Fatalf("unexpected absolute include path: %q", got)
	}

	if issue, ok := validateLocalFileArg("-", resolver, inputset.OSFileSystem{}); ok {
		t.Fatalf("expected stdin path to be ignored, got %+v", issue)
	}
	if issue, ok := validateLocalFileArg("dir", resolver, inputset.OSFileSystem{}); !ok || issue.Code != "expected_file" {
		t.Fatalf("expected expected_file issue, got %+v ok=%v", issue, ok)
	}
	badResolver := inputset.NewWorkspaceResolver(root, root, func(string) (string, error) {
		return "", errors.New("boom")
	})
	if issue, ok := validateLocalFileArg("child.sql", badResolver, inputset.OSFileSystem{}); !ok || issue.Code != "invalid_path" {
		t.Fatalf("expected invalid_path issue, got %+v ok=%v", issue, ok)
	}
	if issues := ValidateArgs([]string{"-f"}, resolver, inputset.OSFileSystem{}); len(issues) != 1 || issues[0].Code != "missing_file_arg" {
		t.Fatalf("unexpected missing-file issues: %+v", issues)
	}
}

func TestPsqlTrackerAndCollectHelpers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.sql"), "select 1;\n")
	resolver := inputset.NewDiffResolver(root)

	if _, err := collectEntryPaths([]string{"-f"}, resolver); err == nil {
		t.Fatalf("expected missing value error")
	}
	if _, err := collectEntryPaths([]string{"-f", "../../outside.sql"}, inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.prep.s9s.yaml"))); err == nil {
		t.Fatalf("expected positional outside-workspace error")
	}
	if _, err := collectEntryPaths([]string{"-f../../outside.sql"}, inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.prep.s9s.yaml"))); err == nil {
		t.Fatalf("expected outside-workspace error")
	}
	if _, err := collectEntryPaths([]string{"--file=../../outside.sql"}, inputset.NewAliasResolver(root, filepath.Join(root, "aliases", "demo.prep.s9s.yaml"))); err == nil {
		t.Fatalf("expected equals outside-workspace error")
	}

	fsWithStatError := hookFS{
		stat: func(string) (fs.FileInfo, error) {
			return nil, errors.New("boom")
		},
	}
	trk := &tracker{
		root:    root,
		baseDir: root,
		seen:    make(map[string]struct{}),
		stack:   make(map[string]struct{}),
		fs:      fsWithStatError,
	}
	var order []string
	if err := trk.collect(filepath.Join(root, "a.sql"), &order); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected stat error, got %v", err)
	}

	fsWithReadError := hookFS{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}
	trk = &tracker{
		root:    root,
		baseDir: root,
		seen:    make(map[string]struct{}),
		stack:   make(map[string]struct{}),
		fs:      fsWithReadError,
	}
	if err := trk.collect(filepath.Join(root, "a.sql"), &order); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected read error, got %v", err)
	}
	order = nil
	trk = &tracker{
		root:    root,
		baseDir: root,
		seen:    make(map[string]struct{}),
		stack:   make(map[string]struct{}),
		fs:      inputset.OSFileSystem{},
	}
	if err := trk.collect(filepath.Join(root, "a.sql"), &order); err != nil {
		t.Fatalf("collect existing file: %v", err)
	}
	if err := trk.collect(filepath.Join(root, "a.sql"), &order); err != nil {
		t.Fatalf("collect seen file: %v", err)
	}
	if len(order) != 1 {
		t.Fatalf("expected seen file to be ignored, got %+v", order)
	}

	finalReadErrFS := hookFS{
		readFile: func() func(string) ([]byte, error) {
			readCount := map[string]int{}
			return func(path string) ([]byte, error) {
				readCount[path]++
				if filepath.Base(path) == "a.sql" && readCount[path] > 1 {
					return nil, errors.New("boom")
				}
				return os.ReadFile(path)
			}
		}(),
	}
	if _, err := Collect([]string{"-fa.sql"}, resolver, finalReadErrFS); err == nil || !strings.Contains(err.Error(), "read") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped final read error, got %v", err)
	}

	if _, _, ok := parseInclude(`\i`); ok {
		t.Fatalf("expected incomplete include command to be ignored")
	}
	if issue, ok := validateLocalFileArg("a.sql", resolver, inputset.OSFileSystem{}); ok {
		t.Fatalf("expected existing file to pass, got %+v", issue)
	}
}
