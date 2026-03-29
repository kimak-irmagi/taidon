package alias

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli/runkind"
)

func TestNormalizeHelpers(t *testing.T) {
	if got := normalizeClass("RUN"); got != ClassRun {
		t.Fatalf("normalizeClass = %q, want %q", got, ClassRun)
	}
	if got := normalizeClasses([]Class{"", ClassPrepare, ClassPrepare, ClassRun}); !reflect.DeepEqual(got, []Class{ClassPrepare, ClassRun}) {
		t.Fatalf("unexpected classes: %+v", got)
	}
	if got := normalizeClasses([]Class{""}); !reflect.DeepEqual(got, []Class{ClassPrepare, ClassRun}) {
		t.Fatalf("unexpected fallback classes: %+v", got)
	}
	if got := normalizeDepth("bad"); got != "" {
		t.Fatalf("normalizeDepth = %q, want empty", got)
	}
	if got := normalizeDepth(""); got != DepthRecursive {
		t.Fatalf("normalizeDepth = %q, want %q", got, DepthRecursive)
	}
	if got := canonicalizeBoundaryPath(""); got != "." {
		t.Fatalf("canonicalizeBoundaryPath(empty) = %q", got)
	}
	if !isWithin(filepath.Join("a", "b"), filepath.Join("a", "b", "c")) {
		t.Fatalf("expected nested path to stay within boundary")
	}
	if isWithin("bad\x00base", "bad\x00target") {
		t.Fatalf("expected invalid path comparison to fail")
	}
}

func TestScanRejectsInvalidInputs(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "outside")
	mkdirAll(t, outside)

	if _, err := Scan(ScanOptions{}); err == nil || !strings.Contains(err.Error(), "workspace root is required") {
		t.Fatalf("expected missing workspace root error, got %v", err)
	}
	if _, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: outside}); err == nil || !strings.Contains(err.Error(), "current working directory") {
		t.Fatalf("expected cwd boundary error, got %v", err)
	}
	if _, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: workspace, Depth: "bad"}); err == nil || !strings.Contains(err.Error(), "invalid scan depth") {
		t.Fatalf("expected invalid depth error, got %v", err)
	}
	filePath := writeAliasFile(t, workspace, "root.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	if _, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: workspace, From: filePath}); err == nil {
		t.Fatalf("expected scan root file error")
	}
}

func TestScanDefaultsCWDToWorkspaceWhenEmpty(t *testing.T) {
	workspace := t.TempDir()
	writeAliasFile(t, workspace, "root.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Ref != "root" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestReadInventoryKindAndHelpers(t *testing.T) {
	if _, err := readInventoryKind(filepath.Join(t.TempDir(), "missing.prep.s9s.yaml"), ClassPrepare); err == nil {
		t.Fatalf("expected missing file error")
	}
	if got := inventoryReadError("", os.ErrInvalid); !strings.Contains(got.Error(), "invalid argument") {
		t.Fatalf("unexpected inventoryReadError: %v", got)
	}
	if got := classifyPath("demo.run.s9s.yaml"); got != ClassRun {
		t.Fatalf("classifyPath = %q, want %q", got, ClassRun)
	}
	if got := classifyPath("demo.txt"); got != "" {
		t.Fatalf("classifyPath(non-alias) = %q, want empty", got)
	}
	if got := workspaceRelativePath(`D:\bad\path`, `C:\bad\root`); got != "path" {
		t.Fatalf("workspaceRelativePath fallback = %q", got)
	}
	if got := workspaceRelativePath(`C:\bad\root\demo.run.s9s.yaml`, `C:\bad\root`); got != "demo.run.s9s.yaml" {
		t.Fatalf("workspaceRelativePath same-drive = %q", got)
	}
	if got := invocationRef(`D:\bad\path\demo.run.s9s.yaml`, `C:\bad\cwd`, ""); got != "demo.run.s9s.yaml" {
		t.Fatalf("invocationRef fallback = %q", got)
	}
	if got := invocationRef(`C:\bad\cwd\demo.run.s9s.yaml`, `C:\bad\cwd`, ClassRun); got != "demo" {
		t.Fatalf("invocationRef same-drive = %q", got)
	}
}

func TestWorkspaceRelativePathAndInvocationRefPreferCanonicalSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "real")
	linkRoot := filepath.Join(root, "link")
	workspace := filepath.Join(realRoot, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	target := writeAliasFile(t, workspace, "schema.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	base := filepath.Join(linkRoot, "workspace")

	if got := workspaceRelativePath(target, base); got != "schema.prep.s9s.yaml" {
		t.Fatalf("workspaceRelativePath = %q, want %q", got, "schema.prep.s9s.yaml")
	}
	if got := invocationRef(target, base, ClassPrepare); got != "schema" {
		t.Fatalf("invocationRef = %q, want %q", got, "schema")
	}
}

func TestResolveTargetErrorPaths(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	outside := filepath.Join(filepath.Dir(workspace), "outside")
	mkdirAll(t, cwd)
	mkdirAll(t, outside)

	if _, err := ResolveTarget(ResolveOptions{}); err == nil || !strings.Contains(err.Error(), "workspace root is required") {
		t.Fatalf("expected missing workspace error, got %v", err)
	}
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd}); err == nil || !strings.Contains(err.Error(), "alias ref is required") {
		t.Fatalf("expected missing ref error, got %v", err)
	}
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "."}); err == nil || !strings.Contains(err.Error(), "alias ref is empty") {
		t.Fatalf("expected empty exact ref error, got %v", err)
	}

	preparePath := writeAliasFile(t, cwd, "mismatch.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "mismatch.prep.s9s.yaml.", Class: ClassRun}); err == nil || !strings.Contains(err.Error(), "does not match file suffix") {
		t.Fatalf("expected class mismatch error, got %v", err)
	}
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "missing", Class: ClassPrepare}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, Ref: "examples/mismatch"}); err != nil {
		t.Fatalf("expected empty cwd to default to workspace, got %v", err)
	}
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: outside, Ref: "mismatch", Class: ClassPrepare}); err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected outside cwd boundary error, got %v", err)
	}

	outsideFile := writeAliasFile(t, outside, "outside.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	_ = outsideFile
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: filepath.ToSlash(outsideFile) + ".", Class: ClassRun}); err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}

	target, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "mismatch.prep.s9s.yaml."})
	if err != nil {
		t.Fatalf("ResolveTarget exact with inferred class: %v", err)
	}
	if target.Class != ClassPrepare || target.File != "examples/mismatch.prep.s9s.yaml" || target.Path != preparePath {
		t.Fatalf("unexpected target: %+v", target)
	}

	dirTarget := filepath.Join(cwd, "scripts")
	mkdirAll(t, dirTarget)
	if _, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "scripts.", Class: ClassRun}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected exact dir target rejection, got %v", err)
	}
}

func TestResolvePathWithinWorkspace(t *testing.T) {
	workspace := t.TempDir()
	rel, err := resolvePathWithinWorkspace("demo.prep.s9s.yaml", workspace, workspace)
	if err != nil {
		t.Fatalf("resolvePathWithinWorkspace: %v", err)
	}
	if rel != filepath.Join(workspace, "demo.prep.s9s.yaml") {
		t.Fatalf("unexpected resolved path: %q", rel)
	}
	rel, err = resolvePathWithinWorkspace("demo.prep.s9s.yaml", workspace, "")
	if err != nil || rel != filepath.Join(workspace, "demo.prep.s9s.yaml") {
		t.Fatalf("expected empty base to default to workspace, got rel=%q err=%v", rel, err)
	}
	if _, err := resolvePathWithinWorkspace("demo.prep.s9s.yaml", "", ""); err == nil || !strings.Contains(err.Error(), "workspace root is required") {
		t.Fatalf("expected missing workspace root error, got %v", err)
	}
}

func TestCheckTargetGuardErrors(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "outside")
	mkdirAll(t, outside)
	aliasPath := writeAliasFile(t, outside, "broken.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	if _, err := CheckTarget(Target{}, ""); err == nil || !strings.Contains(err.Error(), "workspace root is required") {
		t.Fatalf("expected missing workspace root error, got %v", err)
	}
	if _, err := CheckTarget(Target{Class: ClassPrepare, Path: aliasPath}, workspace); err == nil || !strings.Contains(err.Error(), "within workspace root") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
	insideAlias := writeAliasFile(t, workspace, "inside.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	if _, err := CheckTarget(Target{Path: insideAlias}, workspace); err == nil || !strings.Contains(err.Error(), "class is required") {
		t.Fatalf("expected missing class error, got %v", err)
	}
}

func TestCheckPrepareAndRunAliasLoaderErrors(t *testing.T) {
	temp := t.TempDir()

	if _, issues := checkPrepareAlias(filepath.Join(temp, "missing.prep.s9s.yaml"), temp); len(issues) == 0 {
		t.Fatalf("expected prepare loader issue")
	}
	if _, issues := checkRunAlias(filepath.Join(temp, "missing.run.s9s.yaml"), temp); len(issues) == 0 {
		t.Fatalf("expected run loader issue")
	}
	parseErrorRun := writeAliasFile(t, temp, "broken.run.s9s.yaml", "kind: [\n")
	if _, issues := checkRunAlias(parseErrorRun, temp); len(issues) != 1 || !strings.Contains(issues[0].Message, "read run alias") {
		t.Fatalf("expected run parse issue, got %+v", issues)
	}

	cases := []struct {
		name    string
		path    string
		content string
		load    func(string) error
		want    string
	}{
		{
			name:    "prepare kind required",
			path:    "missing-kind.prep.s9s.yaml",
			content: "args:\n  - -c\n  - select 1\n",
			load: func(path string) error {
				_, err := loadPrepareAlias(path)
				return err
			},
			want: "prepare alias kind is required",
		},
		{
			name:    "prepare unknown kind",
			path:    "unknown.prep.s9s.yaml",
			content: "kind: weird\nargs:\n  - -c\n  - select 1\n",
			load: func(path string) error {
				_, err := loadPrepareAlias(path)
				return err
			},
			want: "unknown prepare alias kind",
		},
		{
			name:    "prepare args required",
			path:    "missing-args.prep.s9s.yaml",
			content: "kind: psql\n",
			load: func(path string) error {
				_, err := loadPrepareAlias(path)
				return err
			},
			want: "prepare alias args are required",
		},
		{
			name:    "run kind required",
			path:    "missing-kind.run.s9s.yaml",
			content: "args:\n  - -c\n  - select 1\n",
			load: func(path string) error {
				_, err := loadRunAlias(path)
				return err
			},
			want: "run alias kind is required",
		},
		{
			name:    "run unknown kind",
			path:    "unknown.run.s9s.yaml",
			content: "kind: weird\nargs:\n  - -c\n  - select 1\n",
			load: func(path string) error {
				_, err := loadRunAlias(path)
				return err
			},
			want: "unknown run alias kind",
		},
		{
			name:    "run image unsupported",
			path:    "image.run.s9s.yaml",
			content: "kind: psql\nimage: postgres:17\nargs:\n  - -c\n  - select 1\n",
			load: func(path string) error {
				_, err := loadRunAlias(path)
				return err
			},
			want: "run alias does not support image",
		},
		{
			name:    "run args required",
			path:    "missing-args.run.s9s.yaml",
			content: "kind: psql\n",
			load: func(path string) error {
				_, err := loadRunAlias(path)
				return err
			},
			want: "run alias args are required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAliasFile(t, temp, tc.path, tc.content)
			if err := tc.load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidatePathHelpers(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "folder/demo.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writePlainFile(t, filepath.Join(workspace, "folder"), "script.sql", "select 1;\n")
	mkdirAll(t, filepath.Join(workspace, "folder", "dir"))

	if issues := validatePrepareAliasPaths("weird", nil, aliasPath, workspace); len(issues) != 0 {
		t.Fatalf("unexpected issues for unknown prepare kind: %+v", issues)
	}
	if issues := validatePrepareAliasPaths("lb", []string{"--changelog-file", "missing.xml"}, aliasPath, workspace); len(issues) == 0 {
		t.Fatalf("expected liquibase wrapper issues")
	}
	if issues := validateRunAliasPaths("weird", nil, aliasPath, workspace); len(issues) != 0 {
		t.Fatalf("unexpected issues for unknown run kind: %+v", issues)
	}

	if issues := validateScriptFileArgs([]string{"-f"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_file_arg" {
		t.Fatalf("expected missing_file_arg, got %+v", issues)
	}
	issues := validateScriptFileArgs([]string{"--file=", "-fscript.sql", "--file", "dir", "--file", "-"}, aliasPath, workspace)
	foundExpectedFile := false
	foundEmptyPath := false
	for _, issue := range issues {
		if issue.Code == "expected_file" {
			foundExpectedFile = true
		}
		if issue.Code == "empty_path" {
			foundEmptyPath = true
		}
	}
	if !foundExpectedFile || !foundEmptyPath {
		t.Fatalf("expected expected_file and empty_path issues, got %+v", issues)
	}
	if issues := validateScriptFileArgs([]string{"-fmissing.sql"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path" {
		t.Fatalf("expected compact -f missing path issue, got %+v", issues)
	}

	if issue, ok := validateLocalFileArg("", aliasPath, workspace, true); !ok || issue.Code != "empty_path" {
		t.Fatalf("expected empty_path issue, got %+v ok=%v", issue, ok)
	}
	if issue, ok := validateLocalFileArg("-", aliasPath, workspace, true); ok {
		t.Fatalf("expected stdin path to be ignored, got %+v", issue)
	}
	if issue, ok := validateLocalFileArg(filepath.Join("..", "..", "outside.sql"), aliasPath, workspace, true); !ok || issue.Code != "path_outside_workspace" {
		t.Fatalf("expected outside workspace issue, got %+v ok=%v", issue, ok)
	}
	if issue, ok := validateLocalFileArg("missing.sql", aliasPath, workspace, true); !ok || issue.Code != "missing_path" {
		t.Fatalf("expected missing path issue, got %+v ok=%v", issue, ok)
	}
	if issue, ok := validateLocalFileArg("dir", aliasPath, workspace, true); !ok || issue.Code != "expected_file" {
		t.Fatalf("expected expected_file issue, got %+v ok=%v", issue, ok)
	}
	if issue, ok := validateLocalFileArg("script.sql", aliasPath, workspace, true); ok {
		t.Fatalf("expected existing file to pass, got %+v", issue)
	}
}

func TestValidatePgbenchPathHelpers(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "folder/demo.run.s9s.yaml", "kind: pgbench\nargs:\n  - -T\n  - 30\n")
	writePlainFile(t, filepath.Join(workspace, "folder"), "bench.sql", "select 1;\n")
	writePlainFile(t, filepath.Join(workspace, "folder"), "extra.sql", "select 2;\n")

	if issues := validateRunAliasPaths(runkind.KindPgbench, []string{"-fbench.sql@10", "-T", "30"}, aliasPath, workspace); len(issues) != 0 {
		t.Fatalf("expected weighted pgbench file to pass, got %+v", issues)
	}
	if issues := validateRunAliasPaths(runkind.KindPgbench, []string{"--file=/dev/stdin@3"}, aliasPath, workspace); len(issues) != 0 {
		t.Fatalf("expected stdin pgbench file to pass, got %+v", issues)
	}

	issues := validateRunAliasPaths(runkind.KindPgbench, []string{"-f", "bench.sql", "--file=extra.sql"}, aliasPath, workspace)
	if len(issues) == 0 {
		t.Fatalf("expected duplicate pgbench file issue")
	}
	foundMultiple := false
	for _, issue := range issues {
		if issue.Code == "multiple_file_args" {
			foundMultiple = true
		}
	}
	if !foundMultiple {
		t.Fatalf("expected multiple_file_args issue, got %+v", issues)
	}

	if issues := validateRunAliasPaths(runkind.KindPgbench, []string{"-fmissing.sql@10"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path" {
		t.Fatalf("expected weighted missing-path issue, got %+v", issues)
	}
}

func TestValidateLiquibasePathsAndRemoteRefs(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "db/liquibase.prep.s9s.yaml", "kind: lb\nargs:\n  - update\n")
	writePlainFile(t, filepath.Join(workspace, "db"), "changelog.xml", "<xml/>\n")
	writePlainFile(t, filepath.Join(workspace, "db"), "defaults.properties", "x=y\n")
	mkdirAll(t, filepath.Join(workspace, "db", "migrations"))
	mkdirAll(t, filepath.Join(workspace, "db", "shared"))

	okIssues := validateLiquibasePathArgs([]string{
		"update",
		"--changelog-file=changelog.xml",
		"--defaults-file", "defaults.properties",
		"--searchPath", "migrations,shared",
		"--search-path=classpath:db,https://example.com/db",
	}, aliasPath, workspace)
	if len(okIssues) != 0 {
		t.Fatalf("unexpected liquibase issues: %+v", okIssues)
	}

	if issues := validateLiquibasePathArgs([]string{"--changelog-file"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path_arg" {
		t.Fatalf("expected missing_path_arg, got %+v", issues)
	}
	issues := validateLiquibasePathArgs([]string{
		"--defaults-file=missing.properties",
		"--searchPath=",
		"--searchPath", "migrations,,shared",
	}, aliasPath, workspace)
	if len(issues) < 3 {
		t.Fatalf("expected multiple liquibase issues, got %+v", issues)
	}
	if issues := validateLiquibasePathArgs([]string{"--defaults-file", "missing.properties"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path" {
		t.Fatalf("expected long defaults-file issue, got %+v", issues)
	}
	if issues := validateLiquibasePathArgs([]string{"--searchPath"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path_arg" {
		t.Fatalf("expected missing searchPath arg issue, got %+v", issues)
	}
	if issues := validateLiquibasePathArgs([]string{"--changelog-file=missing.xml"}, aliasPath, workspace); len(issues) != 1 || issues[0].Code != "missing_path" {
		t.Fatalf("expected changelog equals issue, got %+v", issues)
	}

	if got := validateSearchPath("", aliasPath, workspace); len(got) != 1 || got[0].Code != "empty_search_path" {
		t.Fatalf("unexpected empty searchPath issues: %+v", got)
	}
	if got := validateSearchPath("missing-dir", aliasPath, workspace); len(got) != 1 || got[0].Code != "missing_path" {
		t.Fatalf("unexpected missing searchPath issues: %+v", got)
	}
	if issue, ok := validateLocalLiquibaseArg("classpath:db/changelog.xml", aliasPath, workspace, true); ok {
		t.Fatalf("expected classpath ref to be ignored, got %+v", issue)
	}
	if !looksLikeLiquibaseRemoteRef("https://example.com/db") {
		t.Fatalf("expected URL to look like a remote ref")
	}
}

func TestWalkDirectoryPropagatesVisitorError(t *testing.T) {
	root := t.TempDir()
	writeAliasFile(t, root, "one.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	err := walkDirectory(root, 0, DepthRecursive, func(string) error {
		return os.ErrPermission
	})
	if !errorsIs(err, os.ErrPermission) {
		t.Fatalf("expected visitor error, got %v", err)
	}
}

func errorsIs(err error, target error) bool {
	return err != nil && target != nil && strings.Contains(err.Error(), target.Error())
}
