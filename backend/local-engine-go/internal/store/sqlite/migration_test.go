package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigratesParentStateColumn(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := seedLegacySchema(db); err != nil {
		_ = db.Close()
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	verifyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer verifyDB.Close()

	if !hasColumn(t, verifyDB, "states", "parent_state_id") {
		t.Fatalf("expected parent_state_id column after migration")
	}
}

func seedLegacySchema(db *sql.DB) error {
	legacySchema := `
CREATE TABLE IF NOT EXISTS states (
  state_id TEXT PRIMARY KEY,
  state_fingerprint TEXT,
  image_id TEXT NOT NULL,
  prepare_kind TEXT NOT NULL,
  prepare_args_normalized TEXT NOT NULL,
  created_at TEXT NOT NULL,
  size_bytes INTEGER,
  status TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_states_fingerprint ON states(state_fingerprint);
CREATE INDEX IF NOT EXISTS idx_states_image ON states(image_id);
CREATE INDEX IF NOT EXISTS idx_states_kind ON states(prepare_kind);

CREATE TABLE IF NOT EXISTS instances (
  instance_id TEXT PRIMARY KEY,
  state_id TEXT NOT NULL,
  image_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT,
  status TEXT,
  FOREIGN KEY(state_id) REFERENCES states(state_id)
);
CREATE INDEX IF NOT EXISTS idx_instances_state ON instances(state_id);
CREATE INDEX IF NOT EXISTS idx_instances_image ON instances(image_id);
CREATE INDEX IF NOT EXISTS idx_instances_expires ON instances(expires_at);

CREATE TABLE IF NOT EXISTS names (
  name TEXT PRIMARY KEY,
  instance_id TEXT,
  state_id TEXT,
  state_fingerprint TEXT NOT NULL,
  image_id TEXT NOT NULL,
  last_used_at TEXT,
  is_primary INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_names_instance ON names(instance_id);
CREATE INDEX IF NOT EXISTS idx_names_state ON names(state_id);
CREATE INDEX IF NOT EXISTS idx_names_image ON names(image_id);
CREATE INDEX IF NOT EXISTS idx_names_primary ON names(instance_id, is_primary);
`
	_, err := db.Exec(legacySchema)
	return err
}

func hasColumn(t *testing.T, db *sql.DB, tableName, columnName string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()

	var (
		cid       int
		name      string
		colType   string
		notnull   int
		dfltValue sql.NullString
		pk        int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == columnName {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows: %v", err)
	}
	return false
}
