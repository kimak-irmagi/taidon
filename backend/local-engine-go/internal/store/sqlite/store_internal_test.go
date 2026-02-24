package sqlite

import (
	"database/sql"
	"path/filepath"
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

func TestEnsureRuntimeDirColumnDuplicate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE instances (instance_id TEXT PRIMARY KEY, runtime_dir TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := ensureRuntimeDirColumn(db); err != nil {
		t.Fatalf("ensureRuntimeDirColumn: %v", err)
	}
}

func TestEnsureStateCapacityColumnsDuplicate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE states (
		state_id TEXT PRIMARY KEY,
		last_used_at TEXT,
		use_count INTEGER,
		min_retention_until TEXT,
		evicted_at TEXT,
		eviction_reason TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := ensureStateLastUsedAtColumn(db); err != nil {
		t.Fatalf("ensureStateLastUsedAtColumn: %v", err)
	}
	if err := ensureStateUseCountColumn(db); err != nil {
		t.Fatalf("ensureStateUseCountColumn: %v", err)
	}
	if err := ensureStateMinRetentionUntilColumn(db); err != nil {
		t.Fatalf("ensureStateMinRetentionUntilColumn: %v", err)
	}
	if err := ensureStateEvictedAtColumn(db); err != nil {
		t.Fatalf("ensureStateEvictedAtColumn: %v", err)
	}
	if err := ensureStateEvictionReasonColumn(db); err != nil {
		t.Fatalf("ensureStateEvictionReasonColumn: %v", err)
	}
}

func TestInitDBAddsStateCapacityColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE states (
		state_id TEXT PRIMARY KEY,
		state_fingerprint TEXT,
		image_id TEXT NOT NULL,
		prepare_kind TEXT NOT NULL,
		prepare_args_normalized TEXT NOT NULL,
		created_at TEXT NOT NULL,
		size_bytes INTEGER,
		status TEXT
	)`); err != nil {
		t.Fatalf("create states: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE instances (
		instance_id TEXT PRIMARY KEY,
		state_id TEXT NOT NULL,
		image_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT
	)`); err != nil {
		t.Fatalf("create instances: %v", err)
	}

	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	expected := []string{
		"last_used_at",
		"use_count",
		"min_retention_until",
		"evicted_at",
		"eviction_reason",
	}
	for _, name := range expected {
		var found string
		query := `SELECT name FROM pragma_table_info('states') WHERE name = ?`
		if err := db.QueryRow(query, name).Scan(&found); err != nil {
			t.Fatalf("expected states column %q: %v", name, err)
		}
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

func TestInitDBReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatalf("set query_only: %v", err)
	}
	if err := initDB(db); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInitDBSchemaError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	previous := schemaSQL
	schemaSQL = "CREATE TABLE"
	t.Cleanup(func() { schemaSQL = previous })

	if err := initDB(db); err == nil {
		t.Fatalf("expected error")
	}
}

func TestInitDBParentColumnError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE states (state_id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatalf("set query_only: %v", err)
	}
	if err := initDB(db); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnsureColumnsReturnErrorsOnClosedDB(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	checks := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		{name: "ensureRuntimeIDColumn", fn: ensureRuntimeIDColumn},
		{name: "ensureRuntimeDirColumn", fn: ensureRuntimeDirColumn},
		{name: "ensureStateLastUsedAtColumn", fn: ensureStateLastUsedAtColumn},
		{name: "ensureStateUseCountColumn", fn: ensureStateUseCountColumn},
		{name: "ensureStateMinRetentionUntilColumn", fn: ensureStateMinRetentionUntilColumn},
		{name: "ensureStateEvictedAtColumn", fn: ensureStateEvictedAtColumn},
		{name: "ensureStateEvictionReasonColumn", fn: ensureStateEvictionReasonColumn},
	}
	for _, tc := range checks {
		if err := tc.fn(db); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
	}
}
