package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/sqlrs/engine-local/internal/store"
)

func TestStoreStateRetentionFieldsPersisted(t *testing.T) {
	st := openTestStore(t)
	now := time.Date(2026, time.March, 8, 12, 34, 56, 123456789, time.UTC)
	created := now.Format(time.RFC3339Nano)
	lastUsed := now.Add(time.Minute).Format(time.RFC3339Nano)
	minRetention := now.Add(2 * time.Hour).Format(time.RFC3339Nano)

	exec(t, st, `INSERT INTO states (
		state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at,
		size_bytes, last_used_at, use_count, min_retention_until, status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-retained", "parent-1", "fingerprint-1", "image-1", "psql", "args", created,
		int64(512), lastUsed, int64(3), minRetention, nil)

	states, err := st.ListStates(context.Background(), store.StateFilters{IDPrefix: "state-ret"})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one state, got %d", len(states))
	}
	if states[0].ParentStateID == nil || *states[0].ParentStateID != "parent-1" {
		t.Fatalf("unexpected parent id: %+v", states[0].ParentStateID)
	}
	if states[0].LastUsedAt == nil || *states[0].LastUsedAt != lastUsed {
		t.Fatalf("unexpected last used at: %+v", states[0].LastUsedAt)
	}
	if states[0].UseCount == nil || *states[0].UseCount != 3 {
		t.Fatalf("unexpected use count: %+v", states[0].UseCount)
	}
	if states[0].MinRetentionUntil == nil || *states[0].MinRetentionUntil != minRetention {
		t.Fatalf("unexpected min retention: %+v", states[0].MinRetentionUntil)
	}

	entry, ok, err := st.GetState(context.Background(), "state-retained")
	if err != nil || !ok {
		t.Fatalf("GetState: entry=%+v ok=%v err=%v", entry, ok, err)
	}
	if entry.MinRetentionUntil == nil || *entry.MinRetentionUntil != minRetention {
		t.Fatalf("unexpected get-state min retention: %+v", entry.MinRetentionUntil)
	}
}

func TestNewReturnsInitErrorForClosedDB(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := New(db); err == nil {
		t.Fatalf("expected initDB error")
	}
}

func TestCreateInstanceReturnsBeginTxErrorWhenDBClosed(t *testing.T) {
	st := openTestStore(t)
	if err := st.db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := st.CreateInstance(context.Background(), store.InstanceCreate{InstanceID: "instance-1"})
	if err == nil {
		t.Fatalf("expected BeginTx error")
	}
}

func TestParseTimeRFC3339NanoWithFractionalSeconds(t *testing.T) {
	parsed, ok := parseTime("2026-03-08T12:34:56.123456789Z")
	if !ok {
		t.Fatalf("expected RFC3339Nano timestamp to parse")
	}
	if parsed.Nanosecond() != 123456789 {
		t.Fatalf("unexpected nanoseconds: %d", parsed.Nanosecond())
	}
}
