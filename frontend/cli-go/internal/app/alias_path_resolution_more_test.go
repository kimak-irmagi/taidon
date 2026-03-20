package app

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestRebasePrepareAliasArgs(t *testing.T) {
	aliasPath := filepath.Join("workspace", "aliases", "demo.prep.s9s.yaml")

	got := rebasePrepareAliasArgs(" psql ", []string{"-f", "queries/setup.sql"}, aliasPath)
	want := []string{"-f", filepath.Join("workspace", "aliases", "queries", "setup.sql")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebasePrepareAliasArgs(psql) = %#v, want %#v", got, want)
	}

	got = rebasePrepareAliasArgs("lb", []string{"update", "--search-path", "migrations,classpath:db"}, aliasPath)
	want = []string{"update", "--search-path", filepath.Join("workspace", "aliases", "migrations") + ",classpath:db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebasePrepareAliasArgs(lb) = %#v, want %#v", got, want)
	}

	raw := []string{"--flag", "value"}
	got = rebasePrepareAliasArgs("custom", raw, aliasPath)
	if !reflect.DeepEqual(got, raw) {
		t.Fatalf("rebasePrepareAliasArgs(custom) = %#v, want %#v", got, raw)
	}
	if len(got) > 0 && len(raw) > 0 && &got[0] == &raw[0] {
		t.Fatalf("expected copy for unsupported prepare kind")
	}
}

func TestRebaseRunAliasArgs(t *testing.T) {
	aliasPath := filepath.Join("workspace", "aliases", "demo.run.s9s.yaml")

	got := rebaseRunAliasArgs("psql", []string{"--file=query.sql"}, aliasPath)
	want := []string{"--file=" + filepath.Join("workspace", "aliases", "query.sql")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseRunAliasArgs(psql) = %#v, want %#v", got, want)
	}

	got = rebaseRunAliasArgs("pgbench", []string{"-fbench.sql", "-T", "30"}, aliasPath)
	want = []string{"-f" + filepath.Join("workspace", "aliases", "bench.sql"), "-T", "30"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseRunAliasArgs(pgbench) = %#v, want %#v", got, want)
	}

	raw := []string{"-c", "10"}
	got = rebaseRunAliasArgs("custom", raw, aliasPath)
	if !reflect.DeepEqual(got, raw) {
		t.Fatalf("rebaseRunAliasArgs(custom) = %#v, want %#v", got, raw)
	}
	if len(got) > 0 && len(raw) > 0 && &got[0] == &raw[0] {
		t.Fatalf("expected copy for unsupported run kind")
	}
}

func TestRebaseScriptFileArgs(t *testing.T) {
	baseDir := filepath.Join("workspace", "aliases")

	got := rebaseScriptFileArgs([]string{
		"-f", "queries/setup.sql",
		"--file=query.sql",
		"-fbench.sql",
		"--flag", "value",
		"--file",
	}, baseDir)
	want := []string{
		"-f", filepath.Join("workspace", "aliases", "queries", "setup.sql"),
		"--file=" + filepath.Join("workspace", "aliases", "query.sql"),
		"-f" + filepath.Join("workspace", "aliases", "bench.sql"),
		"--flag", "value",
		"--file",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseScriptFileArgs = %#v, want %#v", got, want)
	}
}

func TestRebaseLiquibasePathArgs(t *testing.T) {
	baseDir := filepath.Join("workspace", "aliases")

	got := rebaseLiquibasePathArgs([]string{
		"update",
		"--changelog-file", "migrations/changelog.xml",
		"--defaults-file", "classpath:db/liquibase.properties",
		"--searchPath", "migrations, shared",
		"--changelog-file=https://example.com/changelog.xml",
		"--defaults-file=defaults/liquibase.properties",
		"--searchPath=one,https://example.com/db",
		"--search-path=two,classpath:db",
	}, baseDir)
	want := []string{
		"update",
		"--changelog-file", filepath.Join("workspace", "aliases", "migrations", "changelog.xml"),
		"--defaults-file", "classpath:db/liquibase.properties",
		"--searchPath", filepath.Join("workspace", "aliases", "migrations") + "," + filepath.Join("workspace", "aliases", "shared"),
		"--changelog-file=https://example.com/changelog.xml",
		"--defaults-file=" + filepath.Join("workspace", "aliases", "defaults", "liquibase.properties"),
		"--searchPath=" + filepath.Join("workspace", "aliases", "one") + ",https://example.com/db",
		"--search-path=" + filepath.Join("workspace", "aliases", "two") + ",classpath:db",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseLiquibasePathArgs = %#v, want %#v", got, want)
	}

	got = rebaseLiquibasePathArgs([]string{"update", "--defaults-file"}, baseDir)
	want = []string{"update", "--defaults-file"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseLiquibasePathArgs(missing defaults) = %#v, want %#v", got, want)
	}

	got = rebaseLiquibasePathArgs([]string{"update", "--search-path"}, baseDir)
	want = []string{"update", "--search-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rebaseLiquibasePathArgs(missing search-path) = %#v, want %#v", got, want)
	}
}

func TestRebaseLiquibaseSearchPath(t *testing.T) {
	baseDir := filepath.Join("workspace", "aliases")

	got := rebaseLiquibaseSearchPath(" one , ,classpath:db,https://example.com/db,dir/two ", baseDir)
	want := filepath.Join("workspace", "aliases", "one") + ",,classpath:db,https://example.com/db," + filepath.Join("workspace", "aliases", "dir", "two")
	if got != want {
		t.Fatalf("rebaseLiquibaseSearchPath = %q, want %q", got, want)
	}
}

func TestRebaseAliasFilePath(t *testing.T) {
	baseDir := filepath.Join("workspace", "aliases")
	absPath := filepath.Join(t.TempDir(), "abs.sql")

	if got := rebaseAliasFilePath("   ", baseDir); got != "   " {
		t.Fatalf("rebaseAliasFilePath(blank) = %q", got)
	}
	if got := rebaseAliasFilePath("-", baseDir); got != "-" {
		t.Fatalf("rebaseAliasFilePath(stdin) = %q", got)
	}
	if got := rebaseAliasFilePath(absPath, baseDir); got != filepath.Clean(absPath) {
		t.Fatalf("rebaseAliasFilePath(abs) = %q, want %q", got, filepath.Clean(absPath))
	}
	if got := rebaseAliasFilePath("queries/setup.sql", baseDir); got != filepath.Join("workspace", "aliases", "queries", "setup.sql") {
		t.Fatalf("rebaseAliasFilePath(rel) = %q", got)
	}
}
