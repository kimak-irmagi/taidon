package sqlite

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEnsureParentStateColumnDuplicate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE states (state_id TEXT PRIMARY KEY, parent_state_id TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := ensureParentStateColumn(db); err != nil {
		t.Fatalf("ensureParentStateColumn: %v", err)
	}

	var name string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_states_parent'").Scan(&name); err != nil {
		t.Fatalf("expected index, got %v", err)
	}
}

func TestInitDBClosedDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if err := initDB(db); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnsureParentStateColumnClosedDB(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if err := ensureParentStateColumn(db); err == nil {
		t.Fatalf("expected error")
	}
}
