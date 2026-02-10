package prepare

import (
	"strings"
	"testing"
)

func TestParseLiquibaseUpdateSQLSingleChangeset(t *testing.T) {
	out := strings.Join([]string{
		"-- Liquibase Update SQL",
		"-- Changeset db/changelog/001.sql::1::alice",
		"CREATE TABLE foo (id int);",
		"",
	}, "\n")

	sets, err := parseLiquibaseUpdateSQL(out)
	if err != nil {
		t.Fatalf("parseLiquibaseUpdateSQL: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 changeset, got %d", len(sets))
	}
	cs := sets[0]
	if cs.ID != "1" || cs.Author != "alice" || cs.Path != "db/changelog/001.sql" {
		t.Fatalf("unexpected changeset metadata: %+v", cs)
	}
	if !strings.Contains(cs.SQL, "CREATE TABLE foo") {
		t.Fatalf("unexpected sql payload: %q", cs.SQL)
	}
	expectedHash := sha256Hex(cs.SQL)
	if cs.SQLHash != expectedHash {
		t.Fatalf("unexpected sql_hash: %s", cs.SQLHash)
	}
}

func TestParseLiquibaseUpdateSQLMultipleChangesets(t *testing.T) {
	out := strings.Join([]string{
		"Starting Liquibase at 12:00",
		"-- Changeset db/changelog/001.sql::1::alice",
		"CREATE TABLE foo (id int);",
		"-- Changeset db/changelog/002.sql::2::bob",
		"ALTER TABLE foo ADD COLUMN name text;",
		"",
	}, "\n")

	sets, err := parseLiquibaseUpdateSQL(out)
	if err != nil {
		t.Fatalf("parseLiquibaseUpdateSQL: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("expected 2 changesets, got %d", len(sets))
	}
	if sets[0].ID != "1" || sets[1].ID != "2" {
		t.Fatalf("unexpected order: %+v", sets)
	}
}

func TestParseLiquibaseUpdateSQLFailsWithoutChangesetMarkers(t *testing.T) {
	out := "CREATE TABLE foo (id int);"
	_, err := parseLiquibaseUpdateSQL(out)
	expectValidationError(t, err, "missing changeset")
}

func TestParseLiquibaseUpdateSQLEmptyOutput(t *testing.T) {
	sets, err := parseLiquibaseUpdateSQL("")
	if err != nil {
		t.Fatalf("parseLiquibaseUpdateSQL: %v", err)
	}
	if len(sets) != 0 {
		t.Fatalf("expected no changesets, got %d", len(sets))
	}
}
