package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
)

func TestAnalyzeAliasesRanksRootAndBuildsCopyPasteCommand(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n\\i child.sql\n"))
	mustWriteFile(t, filepath.Join(workspace, "child.sql"), []byte("select 1;\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected one suppressed child candidate, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one root finding, got %+v", report)
	}

	finding := report.Findings[0]
	if finding.Type != alias.ClassPrepare || finding.Kind != "psql" {
		t.Fatalf("unexpected finding: %+v", finding)
	}
	if finding.Ref != "schema" {
		t.Fatalf("unexpected ref: %+v", finding)
	}
	if finding.AliasPath != "schema.prep.s9s.yaml" {
		t.Fatalf("unexpected alias path: %+v", finding)
	}
	if !strings.Contains(finding.CreateCommand, "sqlrs alias create schema prepare:psql -- -f schema.sql") {
		t.Fatalf("unexpected create command: %+v", finding)
	}
}

func TestAnalyzeAliasesSuppressesCoveredAlias(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n"))
	mustWriteFile(t, filepath.Join(workspace, "schema.prep.s9s.yaml"), []byte("kind: psql\nargs:\n  - -f\n  - schema.sql\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected covered candidate to be suppressed, got %+v", report)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for covered alias, got %+v", report)
	}
}

func mustWriteFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
