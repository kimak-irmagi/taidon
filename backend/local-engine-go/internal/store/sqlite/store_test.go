package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/store"
)

func TestStoreListAndGet(t *testing.T) {
	now := time.Now().UTC()
	st := openTestStore(t)
	seedStore(t, st, now)

	ctx := context.Background()

	names, err := st.ListNames(ctx, store.NameFilters{InstanceID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatalf("list names: %v", err)
	}
	if len(names) != 1 || names[0].Name != "dev" {
		t.Fatalf("unexpected names: %+v", names)
	}

	instances, err := st.ListInstances(ctx, store.InstanceFilters{StateID: "state-1"})
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	stateEntries, err := st.ListStates(ctx, store.StateFilters{Kind: "sql"})
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(stateEntries) != 1 || stateEntries[0].StateID != "state-2" || stateEntries[0].RefCount != 1 {
		t.Fatalf("unexpected state entries: %+v", stateEntries)
	}

	name, ok, err := st.GetName(ctx, "ghost")
	if err != nil {
		t.Fatalf("get name: %v", err)
	}
	if !ok || name.Status != store.NameStatusMissing || name.StateID != "state-2" {
		t.Fatalf("unexpected name: %+v", name)
	}

	instance, ok, err := st.GetInstance(ctx, "cccccccccccccccccccccccccccccccc")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !ok || instance.Status != store.InstanceStatusOrphaned {
		t.Fatalf("unexpected instance: %+v", instance)
	}

	state, ok, err := st.GetState(ctx, "state-1")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if !ok || state.RefCount != 2 {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func seedStore(t *testing.T, st *Store, now time.Time) {
	t.Helper()
	created := now.Format(time.RFC3339Nano)
	expiresPast := now.Add(-time.Hour).Format(time.RFC3339Nano)
	expiresFuture := now.Add(time.Hour).Format(time.RFC3339Nano)

	exec(t, st, `INSERT INTO states (state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-1", "state-1", "image-1", "liquibase", "a=1", created, int64(1024), nil)
	exec(t, st, `INSERT INTO states (state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-2", "state-2", "image-2", "sql", "b=2", created, nil, nil)

	exec(t, st, `INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "state-1", "image-1", created, expiresFuture, nil)
	exec(t, st, `INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "state-1", "image-1", created, expiresPast, nil)
	exec(t, st, `INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"cccccccccccccccccccccccccccccccc", "state-2", "image-2", created, nil, nil)

	exec(t, st, `INSERT INTO names (name, instance_id, state_id, state_fingerprint, image_id, last_used_at, is_primary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"dev", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "state-1", "state-1", "image-1", created, 1)
	exec(t, st, `INSERT INTO names (name, instance_id, state_id, state_fingerprint, image_id, last_used_at, is_primary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"qa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "state-1", "state-1", "image-1", created, 1)
	exec(t, st, `INSERT INTO names (name, instance_id, state_id, state_fingerprint, image_id, last_used_at, is_primary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"ghost", nil, nil, "state-2", "image-2", nil, 0)
}

func exec(t *testing.T, st *Store, query string, args ...any) {
	t.Helper()
	if _, err := st.db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
