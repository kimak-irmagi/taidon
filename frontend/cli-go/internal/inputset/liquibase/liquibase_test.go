package liquibase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/inputset"
)

func TestNormalizeArgsRewritesAllPathFlags(t *testing.T) {
	root := t.TempDir()
	resolver := inputset.NewWorkspaceResolver(root, root, nil)

	args, err := NormalizeArgs([]string{
		"update",
		"--changelog-file", "db/changelog.xml",
		"--defaults-file=defaults.properties",
		"--search-path", "db, classpath:conf",
	}, resolver, true)
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}

	want := []string{
		"update",
		"--changelog-file", filepath.Join(root, "db", "changelog.xml"),
		"--defaults-file=" + filepath.Join(root, "defaults.properties"),
		"--searchPath", filepath.Join(root, "db") + ",classpath:conf",
	}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %q, want %q", strings.Join(args, "|"), strings.Join(want, "|"))
	}
}

func TestCollectIncludesDefaultsFileAndYamlChangelog(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "defaults.properties"), "x=y\n")
	writeFile(t, filepath.Join(root, "changelog", "root.yaml"), "databaseChangeLog:\n  - include:\n      file: child.sql\n")
	writeFile(t, filepath.Join(root, "changelog", "child.sql"), "select 1;\n")

	resolver := inputset.NewDiffResolver(root)
	set, err := Collect([]string{
		"--defaults-file", "defaults.properties",
		"--changelog-file", "changelog/root.yaml",
	}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(set.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %+v", set.Entries)
	}
	if set.Entries[0].Path != "defaults.properties" {
		t.Fatalf("entry[0] = %+v", set.Entries[0])
	}
	if set.Entries[1].Path != filepath.ToSlash("changelog/root.yaml") {
		t.Fatalf("entry[1] = %+v", set.Entries[1])
	}
	if set.Entries[2].Path != filepath.ToSlash("changelog/child.sql") {
		t.Fatalf("entry[2] = %+v", set.Entries[2])
	}
}

func TestCollectResolvesRelativeFalseAgainstSearchPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config", "liquibase", "master.xml"), `<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"><include file="child.xml" relativeToChangelogFile="false"/></databaseChangeLog>`)
	writeFile(t, filepath.Join(root, "shared", "child.xml"), `<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog"/>`)

	resolver := inputset.NewDiffResolver(root)
	set, err := Collect([]string{
		"--searchPath", "shared",
		"--changelog-file", "config/liquibase/master.xml",
	}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(set.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %+v", set.Entries)
	}
	if set.Entries[1].Path != filepath.ToSlash("shared/child.xml") {
		t.Fatalf("entry[1] = %+v", set.Entries[1])
	}
}

func TestCollectInvocationInputsIncludesDefaultsFileWithoutChangelog(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "defaults.properties"), "x=y\n")

	resolver := inputset.NewWorkspaceResolver(root, root, nil)
	set, err := CollectInvocationInputs([]string{
		"update",
		"--defaults-file", "defaults.properties",
	}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("CollectInvocationInputs: %v", err)
	}
	if len(set.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %+v", set.Entries)
	}
	if set.Entries[0].Path != "defaults.properties" {
		t.Fatalf("entry[0] = %+v", set.Entries[0])
	}
}

func TestCollectInvocationInputsIncludesSecondaryLocalAssets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "changelog", "master.xml"), strings.Join([]string{
		`<databaseChangeLog xmlns="http://www.liquibase.org/xml/ns/dbchangelog">`,
		`  <changeSet id="1" author="dev">`,
		`    <sqlFile path="seed.sql" relativeToChangelogFile="true"/>`,
		`    <loadData file="seed.csv" relativeToChangelogFile="true"/>`,
		`  </changeSet>`,
		`</databaseChangeLog>`,
	}, "\n"))
	writeFile(t, filepath.Join(root, "changelog", "seed.sql"), "select 1;\n")
	writeFile(t, filepath.Join(root, "changelog", "seed.csv"), "id,name\n1,test\n")

	resolver := inputset.NewWorkspaceResolver(root, root, nil)
	set, err := CollectInvocationInputs([]string{
		"update",
		"--changelog-file", "changelog/master.xml",
	}, resolver, inputset.OSFileSystem{})
	if err != nil {
		t.Fatalf("CollectInvocationInputs: %v", err)
	}

	if len(set.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %+v", set.Entries)
	}
	if set.Entries[0].Path != filepath.ToSlash("changelog/master.xml") {
		t.Fatalf("entry[0] = %+v", set.Entries[0])
	}
	if set.Entries[1].Path != filepath.ToSlash("changelog/seed.sql") {
		t.Fatalf("entry[1] = %+v", set.Entries[1])
	}
	if set.Entries[2].Path != filepath.ToSlash("changelog/seed.csv") {
		t.Fatalf("entry[2] = %+v", set.Entries[2])
	}
}

func TestValidateArgsAccumulatesLiquibaseIssues(t *testing.T) {
	root := t.TempDir()
	aliasPath := filepath.Join(root, "aliases", "demo.prep.s9s.yaml")
	if err := os.MkdirAll(filepath.Join(filepath.Dir(aliasPath), "migrations"), 0o700); err != nil {
		t.Fatalf("MkdirAll migrations: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(aliasPath), "shared"), 0o700); err != nil {
		t.Fatalf("MkdirAll shared: %v", err)
	}
	resolver := inputset.NewAliasResolver(root, aliasPath)

	issues := ValidateArgs([]string{
		"--defaults-file=missing.properties",
		"--searchPath=",
		"--searchPath", "migrations,,shared",
	}, resolver, inputset.OSFileSystem{})
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %+v", issues)
	}
	if issues[0].Code != "missing_path" || issues[1].Code != "empty_search_path" || issues[2].Code != "empty_search_path_item" {
		t.Fatalf("unexpected issues: %+v", issues)
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
