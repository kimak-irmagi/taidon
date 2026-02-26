package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/store"
)

func TestStoreCreateAndDelete(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	parentID := "state-parent"
	stateID := "state-1"
	if err := st.CreateState(ctx, store.StateCreate{
		StateID:               stateID,
		ParentStateID:         &parentID,
		StateFingerprint:      stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-f file.sql",
		CreatedAt:             now,
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}

	state, ok, err := st.GetState(ctx, stateID)
	if err != nil || !ok {
		t.Fatalf("GetState: %v ok=%v", err, ok)
	}
	if state.ParentStateID == nil || *state.ParentStateID != parentID {
		t.Fatalf("unexpected parent state id: %+v", state.ParentStateID)
	}
	if state.LastUsedAt == nil || *state.LastUsedAt != now {
		t.Fatalf("expected state last_used_at to be initialized, got %+v", state.LastUsedAt)
	}
	if state.UseCount == nil || *state.UseCount != 0 {
		t.Fatalf("expected state use_count=0, got %+v", state.UseCount)
	}

	instanceID := strings.Repeat("a", 32)
	if err := st.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    "image-1",
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	instance, ok, err := st.GetInstance(ctx, instanceID)
	if err != nil || !ok {
		t.Fatalf("GetInstance: %v ok=%v", err, ok)
	}
	if instance.StateID != stateID {
		t.Fatalf("unexpected instance state: %+v", instance)
	}
	state, ok, err = st.GetState(ctx, stateID)
	if err != nil || !ok {
		t.Fatalf("GetState after instance create: %v ok=%v", err, ok)
	}
	if state.LastUsedAt == nil || *state.LastUsedAt != now {
		t.Fatalf("expected state last_used_at=%s, got %+v", now, state.LastUsedAt)
	}
	if state.UseCount == nil || *state.UseCount != 1 {
		t.Fatalf("expected state use_count=1, got %+v", state.UseCount)
	}

	if err := st.DeleteInstance(ctx, instanceID); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if _, ok, err := st.GetInstance(ctx, instanceID); err != nil || ok {
		t.Fatalf("expected instance deleted, err=%v ok=%v", err, ok)
	}

	if err := st.DeleteState(ctx, stateID); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if _, ok, err := st.GetState(ctx, stateID); err != nil || ok {
		t.Fatalf("expected state deleted, err=%v ok=%v", err, ok)
	}
}

func TestCreateInstanceInsertError(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	err := st.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: "ffffffffffffffffffffffffffffffff",
		StateID:    "missing-state",
		ImageID:    "image-1",
		CreatedAt:  now,
	})
	if err == nil {
		t.Fatalf("expected insert error for missing state")
	}

	var count int
	if err := st.db.QueryRow(`SELECT COUNT(1) FROM instances WHERE instance_id = ?`, "ffffffffffffffffffffffffffffffff").Scan(&count); err != nil {
		t.Fatalf("count instances: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no inserted instance after failure, got %d", count)
	}
}

func TestCreateInstanceUpdateErrorRollsBackTransaction(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	exec(t, st, `INSERT INTO states (state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at, use_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"state-1", "state-1", "image-1", "psql", "args", now, 0)
	exec(t, st, `CREATE TRIGGER fail_state_usage_update
BEFORE UPDATE OF last_used_at, use_count ON states
BEGIN
	SELECT RAISE(ABORT, 'boom');
END`)

	err := st.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: "11111111111111111111111111111111",
		StateID:    "state-1",
		ImageID:    "image-1",
		CreatedAt:  now,
	})
	if err == nil {
		t.Fatalf("expected update error")
	}

	var count int
	if err := st.db.QueryRow(`SELECT COUNT(1) FROM instances WHERE instance_id = ?`, "11111111111111111111111111111111").Scan(&count); err != nil {
		t.Fatalf("count instances: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback for failed update, got instances=%d", count)
	}
}

func TestAddPrefixFilter(t *testing.T) {
	var query strings.Builder
	args := []any{}
	addPrefixFilter(&query, &args, "col", " AbCd ")
	if !strings.Contains(query.String(), "lower(col) LIKE ?") {
		t.Fatalf("unexpected query: %q", query.String())
	}
	if len(args) != 1 || args[0] != "abcd%" {
		t.Fatalf("unexpected args: %+v", args)
	}
}

func TestParseTime(t *testing.T) {
	if _, ok := parseTime("  "); ok {
		t.Fatalf("expected empty time to be invalid")
	}
	now := time.Now().UTC()
	if parsed, ok := parseTime(now.Format(time.RFC3339Nano)); !ok || parsed.IsZero() {
		t.Fatalf("expected RFC3339Nano to parse")
	}
	if parsed, ok := parseTime(now.Format(time.RFC3339)); !ok || parsed.IsZero() {
		t.Fatalf("expected RFC3339 to parse")
	}
	if _, ok := parseTime("nope"); ok {
		t.Fatalf("expected invalid time")
	}
}

func TestOpenEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestOpenInvalidDirectory(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := Open(filepath.Join(blocker, "state.db")); err == nil {
		t.Fatalf("expected error for invalid directory")
	}
}

func TestOpenDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := Open(path); err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func TestStoreCloseNil(t *testing.T) {
	var st *Store
	if err := st.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now().UTC()
	expired := sql.NullString{String: now.Add(-time.Hour).Format(time.RFC3339Nano), Valid: true}
	if !isExpired(expired, now) {
		t.Fatalf("expected expired")
	}
	future := sql.NullString{String: now.Add(time.Hour).Format(time.RFC3339Nano), Valid: true}
	if isExpired(future, now) {
		t.Fatalf("expected not expired")
	}
	invalid := sql.NullString{String: "nope", Valid: true}
	if isExpired(invalid, now) {
		t.Fatalf("expected invalid time to be treated as not expired")
	}
	empty := sql.NullString{String: "", Valid: true}
	if isExpired(empty, now) {
		t.Fatalf("expected empty time to be treated as not expired")
	}
}
