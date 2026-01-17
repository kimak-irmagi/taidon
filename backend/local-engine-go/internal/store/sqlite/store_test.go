package sqlite

import (
	"context"
	"path/filepath"
	"strings"
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

func TestStoreStatusFlags(t *testing.T) {
	now := time.Now().UTC()
	st := openTestStore(t)
	seedStore(t, st, now)

	ctx := context.Background()
	name, ok, err := st.GetName(ctx, "qa")
	if err != nil {
		t.Fatalf("get name: %v", err)
	}
	if !ok || name.Status != store.NameStatusExpired {
		t.Fatalf("unexpected name status: %+v", name)
	}

	instance, ok, err := st.GetInstance(ctx, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !ok || instance.Status != store.InstanceStatusExpired {
		t.Fatalf("unexpected instance status: %+v", instance)
	}

	instance, ok, err = st.GetInstance(ctx, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !ok || instance.Status != store.InstanceStatusActive {
		t.Fatalf("unexpected instance status: %+v", instance)
	}
}

func TestStoreGetMissingEntries(t *testing.T) {
	st := openTestStore(t)

	if _, ok, err := st.GetName(context.Background(), "missing"); err != nil || ok {
		t.Fatalf("expected missing name, err=%v ok=%v", err, ok)
	}
	if _, ok, err := st.GetInstance(context.Background(), "missing"); err != nil || ok {
		t.Fatalf("expected missing instance, err=%v ok=%v", err, ok)
	}
	if _, ok, err := st.GetState(context.Background(), "missing"); err != nil || ok {
		t.Fatalf("expected missing state, err=%v ok=%v", err, ok)
	}
}

func TestStoreListNamesStatuses(t *testing.T) {
	now := time.Now().UTC()
	st := openTestStore(t)
	seedStore(t, st, now)

	entries, err := st.ListNames(context.Background(), store.NameFilters{})
	if err != nil {
		t.Fatalf("ListNames: %v", err)
	}
	got := map[string]store.NameEntry{}
	for _, entry := range entries {
		got[entry.Name] = entry
	}
	if got["dev"].Status != store.NameStatusActive {
		t.Fatalf("unexpected dev status: %+v", got["dev"])
	}
	if got["qa"].Status != store.NameStatusExpired {
		t.Fatalf("unexpected qa status: %+v", got["qa"])
	}
	if got["ghost"].Status != store.NameStatusMissing {
		t.Fatalf("unexpected ghost status: %+v", got["ghost"])
	}
}

func TestStoreListInstancesStatuses(t *testing.T) {
	now := time.Now().UTC()
	st := openTestStore(t)
	seedStore(t, st, now)

	entries, err := st.ListInstances(context.Background(), store.InstanceFilters{StateID: "state-1"})
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	got := map[string]store.InstanceEntry{}
	for _, entry := range entries {
		got[entry.InstanceID] = entry
	}
	if got["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"].Status != store.InstanceStatusActive {
		t.Fatalf("unexpected active status: %+v", got["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])
	}
	if got["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"].Status != store.InstanceStatusExpired {
		t.Fatalf("unexpected expired status: %+v", got["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"])
	}

	orphaned, err := st.ListInstances(context.Background(), store.InstanceFilters{StateID: "state-2"})
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(orphaned) != 1 || orphaned[0].Status != store.InstanceStatusOrphaned {
		t.Fatalf("unexpected orphaned instances: %+v", orphaned)
	}
}

func TestStoreQueryErrors(t *testing.T) {
	st := openTestStore(t)
	if err := st.db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if _, err := st.ListNames(context.Background(), store.NameFilters{}); err == nil {
		t.Fatalf("expected ListNames error")
	}
	if _, err := st.ListInstances(context.Background(), store.InstanceFilters{}); err == nil {
		t.Fatalf("expected ListInstances error")
	}
	if _, err := st.ListStates(context.Background(), store.StateFilters{}); err == nil {
		t.Fatalf("expected ListStates error")
	}
	if _, _, err := st.GetInstance(context.Background(), "inst"); err == nil {
		t.Fatalf("expected GetInstance error")
	}
	if _, _, err := st.GetState(context.Background(), "state"); err == nil {
		t.Fatalf("expected GetState error")
	}
}

func TestAddFilter(t *testing.T) {
	var query strings.Builder
	args := []any{}
	addFilter(&query, &args, "col", " ")
	if query.Len() != 0 || len(args) != 0 {
		t.Fatalf("expected empty query, got %q args=%v", query.String(), args)
	}
	addFilter(&query, &args, "col", " value ")
	if !strings.Contains(query.String(), "col = ?") {
		t.Fatalf("unexpected query: %q", query.String())
	}
	if len(args) != 1 || args[0] != "value" {
		t.Fatalf("unexpected args: %+v", args)
	}
}

func TestGetNameReturnsErrorOnMissingTable(t *testing.T) {
	st := openTestStore(t)
	exec(t, st, "DROP TABLE names")

	if _, _, err := st.GetName(context.Background(), "dev"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestListStatesParentFilter(t *testing.T) {
	st := openTestStore(t)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	exec(t, st, `INSERT INTO states (state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "state-parent", nil, "state-parent", "image-1", "psql", "args", now)
	exec(t, st, `INSERT INTO states (state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "state-child", "state-parent", "state-child", "image-1", "psql", "args", now)

	states, err := st.ListStates(context.Background(), store.StateFilters{ParentID: "state-parent"})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(states) != 1 || states[0].StateID != "state-child" {
		t.Fatalf("unexpected states: %+v", states)
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
