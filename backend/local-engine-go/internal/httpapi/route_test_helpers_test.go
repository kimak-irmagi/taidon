package httpapi

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/sqlrs/engine-local/internal/conntrack"
	"github.com/sqlrs/engine-local/internal/deletion"
	"github.com/sqlrs/engine-local/internal/registry"
	"github.com/sqlrs/engine-local/internal/store/sqlite"
)

func newRouteTestOptions(t *testing.T) (Options, func()) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := seedHTTPData(db); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	deleteMgr, err := deletion.NewManager(deletion.Options{
		Store: st,
		Conn:  conntrack.Noop{},
	})
	if err != nil {
		t.Fatalf("delete manager: %v", err)
	}

	opts := Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   registry.New(st),
		Prepare:    newPrepareManager(t, st, mustOpenQueue(t, dbPath)),
		Deletion:   deleteMgr,
		Config:     &fakeConfig{schema: map[string]any{"type": "object"}},
	}

	cleanup := func() {
		_ = db.Close()
		_ = st.Close()
	}
	return opts, cleanup
}
