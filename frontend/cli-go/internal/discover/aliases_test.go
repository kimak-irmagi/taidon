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

func TestAnalyzeAliasesSkipsUnsupportedExtensions(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "README.md"), []byte("create table users(id int);\nselect 1;\n"))
	mustWriteFile(t, filepath.Join(workspace, "run.sh"), []byte("select 1;\n"))
	mustWriteFile(t, filepath.Join(workspace, "notes.txt"), []byte("create table users(id int);\n"))
	mustWriteFile(t, filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Prefiltered != 1 {
		t.Fatalf("expected one supported candidate, got %+v", report)
	}
	if report.Validated != 1 {
		t.Fatalf("expected one validated candidate, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one finding, got %+v", report)
	}

	finding := report.Findings[0]
	if finding.File != "schema.sql" {
		t.Fatalf("unexpected finding file: %+v", finding)
	}
	if finding.Type != alias.ClassPrepare || finding.Kind != "psql" {
		t.Fatalf("unexpected finding kind: %+v", finding)
	}
}

func TestAnalyzeAliasesSuppressesLiquibaseClosureCoveredByAlias(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app.prep.s9s.yaml"), []byte(`kind: lb
args:
  - update
  - --searchPath
  - liquibase/jhipster-sample-app
  - --changelog-file
  - liquibase/jhipster-sample-app/config/liquibase/master.xml
`))
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app", "config", "liquibase", "master.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
    <include file="config/liquibase/changelog/00000000000000_initial_schema.xml" relativeToChangelogFile="false"/>
</databaseChangeLog>
`))
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app", "config", "liquibase", "changelog", "00000000000000_initial_schema.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"></databaseChangeLog>
`))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 2 {
		t.Fatalf("expected master and child candidates to be suppressed, got %+v", report)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for covered liquibase tree, got %+v", report)
	}
}

func TestAnalyzeAliasesRecognizesLiquibaseProjectRoot(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app", "config", "liquibase", "master.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
    <include file="config/liquibase/changelog/00000000000000_initial_schema.xml" relativeToChangelogFile="false"/>
</databaseChangeLog>
`))
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "jhipster-sample-app", "config", "liquibase", "changelog", "00000000000000_initial_schema.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"></databaseChangeLog>
`))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected child changelog to be suppressed by topology, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one liquibase root finding, got %+v", report)
	}

	finding := report.Findings[0]
	if finding.Type != alias.ClassPrepare || finding.Kind != "lb" {
		t.Fatalf("unexpected liquibase finding: %+v", finding)
	}
	if finding.Ref != "liquibase/jhipster-sample-app" {
		t.Fatalf("unexpected liquibase ref: %+v", finding)
	}
	if finding.AliasPath != filepath.ToSlash(filepath.Join("liquibase", "jhipster-sample-app.prep.s9s.yaml")) {
		t.Fatalf("unexpected liquibase alias path: %+v", finding)
	}
	if !finding.Valid {
		t.Fatalf("expected liquibase finding to be valid: %+v", finding)
	}
	if !strings.Contains(finding.CreateCommand, "--searchPath liquibase/jhipster-sample-app") {
		t.Fatalf("expected liquibase create command to include searchPath, got %+v", finding)
	}
}

func TestAnalyzeAliasesSkipsUnrelatedBinaryArtifactsAndValidatesLiquibaseJar(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "build", "app.jar"), []byte("binary payload"))
	mustWriteFile(t, filepath.Join(workspace, "build", "Worker.class"), []byte("binary payload"))
	mustWriteFile(t, filepath.Join(workspace, "liquibase", "master.jar"), []byte("binary payload"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Prefiltered != 1 {
		t.Fatalf("expected only the Liquibase-named jar to be promoted, got %+v", report)
	}
	if report.Validated != 1 {
		t.Fatalf("expected one validated binary candidate, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one finding, got %+v", report)
	}

	finding := report.Findings[0]
	if finding.Type != alias.ClassPrepare || finding.Kind != "lb" {
		t.Fatalf("unexpected binary Liquibase finding: %+v", finding)
	}
	if !finding.Valid {
		t.Fatalf("expected binary Liquibase finding to be valid: %+v", finding)
	}
	if finding.File != filepath.ToSlash(filepath.Join("liquibase", "master.jar")) {
		t.Fatalf("unexpected binary Liquibase file: %+v", finding)
	}
	if finding.AliasPath != "liquibase.prep.s9s.yaml" {
		t.Fatalf("unexpected binary Liquibase alias path: %+v", finding)
	}
	if !strings.Contains(finding.CreateCommand, "sqlrs alias create liquibase prepare:lb -- update --changelog-file liquibase/master.jar") {
		t.Fatalf("unexpected binary Liquibase create command: %+v", finding)
	}
}

func TestAnalyzeAliasesValidatesNestedPsqlAndPgbenchCandidates(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "team", "workloads", "schema.sql"), []byte("create table users(id int);\n"))
	mustWriteFile(t, filepath.Join(workspace, "team", "perf", "bench.sql"), []byte("select 1;\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected two findings, got %+v", report)
	}

	findings := make(map[string]Finding, len(report.Findings))
	for _, finding := range report.Findings {
		findings[finding.AliasPath] = finding
	}

	psqlFinding, ok := findings[filepath.ToSlash(filepath.Join("team", "workloads.prep.s9s.yaml"))]
	if !ok {
		t.Fatalf("missing nested psql finding: %+v", report.Findings)
	}
	if !psqlFinding.Valid {
		t.Fatalf("expected nested psql finding to be valid: %+v", psqlFinding)
	}
	if !strings.Contains(psqlFinding.CreateCommand, "sqlrs alias create team/workloads prepare:psql -- -f team/workloads/schema.sql") {
		t.Fatalf("unexpected nested psql create command: %+v", psqlFinding)
	}

	pgbenchFinding, ok := findings[filepath.ToSlash(filepath.Join("team", "perf.run.s9s.yaml"))]
	if !ok {
		t.Fatalf("missing nested pgbench finding: %+v", report.Findings)
	}
	if !pgbenchFinding.Valid {
		t.Fatalf("expected nested pgbench finding to be valid: %+v", pgbenchFinding)
	}
	if !strings.Contains(pgbenchFinding.CreateCommand, "sqlrs alias create team/perf run:pgbench -- -f team/perf/bench.sql") {
		t.Fatalf("unexpected nested pgbench create command: %+v", pgbenchFinding)
	}
}

func TestAnalyzeAliasesDeduplicatesAliasPathFindings(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "batch", "001_init.sql"), []byte("create table users(id int);\n"))
	mustWriteFile(t, filepath.Join(workspace, "batch", "002_users.sql"), []byte("create table orders(id int);\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Suppressed != 1 {
		t.Fatalf("expected one duplicate suppression, got %+v", report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one deduplicated finding, got %+v", report)
	}

	finding := report.Findings[0]
	if finding.AliasPath != filepath.ToSlash("batch.prep.s9s.yaml") {
		t.Fatalf("unexpected alias path: %+v", finding)
	}
	if finding.File != filepath.ToSlash(filepath.Join("batch", "001_init.sql")) {
		t.Fatalf("unexpected winning finding: %+v", finding)
	}
}

func TestAnalyzeAliasesCountsAllScannedFiles(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "README.md"), []byte("create table ignored(id int);\n"))
	mustWriteFile(t, filepath.Join(workspace, "notes.txt"), []byte("select 1;\n"))
	mustWriteFile(t, filepath.Join(workspace, "schema.sql"), []byte("create table users(id int);\n"))

	report, err := AnalyzeAliases(Options{WorkspaceRoot: workspace, CWD: workspace})
	if err != nil {
		t.Fatalf("AnalyzeAliases: %v", err)
	}
	if report.Scanned != 3 {
		t.Fatalf("expected all visited files to be counted, got %+v", report)
	}
	if report.Prefiltered != 1 || report.Validated != 1 || len(report.Findings) != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
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
