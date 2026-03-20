package alias

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDefaultsToCWDRecursive(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "local.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, filepath.Join("nested", "smoke.run.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, workspace, "root.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Ref != "local" || entries[1].Ref != "nested/smoke" {
		t.Fatalf("unexpected refs: %+v", entries)
	}
}

func TestScanFromWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, workspace, "root.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, "child.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, From: "workspace"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].File != "examples/child.run.s9s.yaml" || entries[1].File != "root.prep.s9s.yaml" {
		t.Fatalf("unexpected files: %+v", entries)
	}
}

func TestScanFromExplicitPathRelativeToCWD(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	root := filepath.Join(cwd, "nested")
	mkdirAll(t, root)
	writeAliasFile(t, root, "child.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, workspace, "root.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, From: "nested"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Ref != "nested/child" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanRejectsRootOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	outside := filepath.Join(filepath.Dir(workspace), "outside")
	mkdirAll(t, cwd)
	mkdirAll(t, outside)

	_, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, From: outside})
	if err == nil || !strings.Contains(err.Error(), "within workspace") {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
}

func TestScanDepthSelf(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "self.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, filepath.Join("child", "nested.run.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, Depth: string(DepthSelf)})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Ref != "self" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanDepthChildren(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "self.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, filepath.Join("child", "direct.run.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, filepath.Join("child", "grand", "deep.prep.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, Depth: string(DepthChildren)})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Ref != "child/direct" || entries[1].Ref != "self" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanFiltersPrepareOnly(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "one.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, "two.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, Classes: []Class{ClassPrepare}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Class != ClassPrepare {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanFiltersRunOnly(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "one.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, "two.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, Classes: []Class{ClassRun}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Class != ClassRun {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanSortsByWorkspaceRelativePath(t *testing.T) {
	workspace := t.TempDir()
	cwd := workspace
	writeAliasFile(t, workspace, filepath.Join("b", "second.run.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, workspace, filepath.Join("a", "first.prep.s9s.yaml"), "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd, From: "workspace"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].File != "a/first.prep.s9s.yaml" || entries[1].File != "b/second.run.s9s.yaml" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanSkipsSqlrsDirectory(t *testing.T) {
	workspace := t.TempDir()
	cwd := workspace
	writeAliasFile(t, filepath.Join(workspace, ".sqlrs"), "hidden.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, workspace, "visible.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 || entries[0].Ref != "visible" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestScanKeepsMalformedAliasEntries(t *testing.T) {
	workspace := t.TempDir()
	cwd := workspace
	writeAliasFile(t, workspace, "broken.run.s9s.yaml", "kind: [\n")

	entries, err := Scan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0].Kind != "" || entries[0].Error == "" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestResolveTargetPrepareStem(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	target, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "chinook", Class: ClassPrepare})
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if target.Class != ClassPrepare || !strings.HasSuffix(target.Path, "chinook.prep.s9s.yaml") {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveTargetRunStem(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	target, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "smoke", Class: ClassRun})
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if target.Class != ClassRun || !strings.HasSuffix(target.Path, "smoke.run.s9s.yaml") {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveTargetRejectsAmbiguousStem(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, cwd)
	writeAliasFile(t, cwd, "foo.prep.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")
	writeAliasFile(t, cwd, "foo.run.s9s.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	_, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "foo"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestResolveTargetAcceptsExactFileEscapeWithSelector(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, filepath.Join(cwd, "scripts"))
	writeAliasFile(t, filepath.Join(cwd, "scripts"), "smoke.alias.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	target, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "scripts/smoke.alias.yaml.", Class: ClassRun})
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if target.Class != ClassRun || !strings.HasSuffix(target.Path, "smoke.alias.yaml") {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestResolveTargetRejectsExactFileEscapeWithoutSelectorForNonstandardFile(t *testing.T) {
	workspace := t.TempDir()
	cwd := filepath.Join(workspace, "examples")
	mkdirAll(t, filepath.Join(cwd, "scripts"))
	writeAliasFile(t, filepath.Join(cwd, "scripts"), "smoke.alias.yaml", "kind: psql\nargs:\n  - -c\n  - select 1\n")

	_, err := ResolveTarget(ResolveOptions{WorkspaceRoot: workspace, CWD: cwd, Ref: "scripts/smoke.alias.yaml."})
	if err == nil || !strings.Contains(err.Error(), "--prepare") {
		t.Fatalf("expected selector error, got %v", err)
	}
}

func TestCheckPrepareAliasSuccess(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - queries.sql\n")
	writePlainFile(t, workspace, "queries.sql", "select 1;\n")

	result, err := CheckTarget(Target{Class: ClassPrepare, Path: aliasPath}, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if !result.Valid || result.Error != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckRunAliasSuccess(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - smoke.sql\n")
	writePlainFile(t, workspace, "smoke.sql", "select 1;\n")

	result, err := CheckTarget(Target{Class: ClassRun, Path: aliasPath}, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if !result.Valid || result.Error != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckPrepareAliasMissingReferencedFile(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "chinook.prep.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - missing.sql\n")

	result, err := CheckTarget(Target{Class: ClassPrepare, Path: aliasPath}, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if result.Valid || !strings.Contains(result.Error, "missing.sql") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckRunAliasMissingReferencedFile(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "smoke.run.s9s.yaml", "kind: pgbench\nargs:\n  - -f\n  - scripts/missing.sql\n")

	result, err := CheckTarget(Target{Class: ClassRun, Path: aliasPath}, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if result.Valid || !strings.Contains(result.Error, "scripts/missing.sql") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckReportsParseErrorAsInvalid(t *testing.T) {
	workspace := t.TempDir()
	aliasPath := writeAliasFile(t, workspace, "broken.prep.s9s.yaml", "kind: [\n")

	result, err := CheckTarget(Target{Class: ClassPrepare, Path: aliasPath}, workspace)
	if err != nil {
		t.Fatalf("CheckTarget: %v", err)
	}
	if result.Valid || !strings.Contains(result.Error, "read prepare alias") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheckAggregatesResultsInScanMode(t *testing.T) {
	workspace := t.TempDir()
	cwd := workspace
	writeAliasFile(t, workspace, "ok.prep.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - ok.sql\n")
	writePlainFile(t, workspace, "ok.sql", "select 1;\n")
	writeAliasFile(t, workspace, "bad.run.s9s.yaml", "kind: psql\nargs:\n  - -f\n  - missing.sql\n")

	report, err := CheckScan(ScanOptions{WorkspaceRoot: workspace, CWD: cwd})
	if err != nil {
		t.Fatalf("CheckScan: %v", err)
	}
	if report.Checked != 2 || report.ValidCount != 1 || report.InvalidCount != 1 {
		t.Fatalf("unexpected summary: %+v", report)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeAliasFile(t *testing.T, dir, relPath, contents string) string {
	t.Helper()
	path := filepath.Join(dir, relPath)
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	return path
}

func writePlainFile(t *testing.T, dir, relPath, contents string) string {
	t.Helper()
	path := filepath.Join(dir, relPath)
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write plain file: %v", err)
	}
	return path
}
