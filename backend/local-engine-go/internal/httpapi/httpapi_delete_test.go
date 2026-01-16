package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"sqlrs/engine/internal/conntrack"
	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/store/sqlite"
)

type fakeConnTracker struct {
	counts map[string]int
}

func (f fakeConnTracker) ActiveConnections(ctx context.Context, instanceID string) (int, error) {
	if f.counts == nil {
		return 0, nil
	}
	return f.counts[instanceID], nil
}

func TestDeleteInstanceDryRunWouldDelete(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedInstanceData, fakeConnTracker{})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/instances/inst-1?dry_run=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result deletion.DeleteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.DryRun || result.Outcome != deletion.OutcomeWouldDelete {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Root.Kind != "instance" || result.Root.ID != "inst-1" {
		t.Fatalf("unexpected root: %+v", result.Root)
	}
	if result.Root.Connections == nil || *result.Root.Connections != 0 {
		t.Fatalf("expected connections=0, got %+v", result.Root.Connections)
	}
}

func TestDeleteInstanceBlockedWithoutForce(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedInstanceData, fakeConnTracker{counts: map[string]int{"inst-1": 2}})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/instances/inst-1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	var result deletion.DeleteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Outcome != deletion.OutcomeBlocked {
		t.Fatalf("unexpected outcome: %+v", result)
	}
	if result.Root.Blocked != deletion.BlockActiveConnections {
		t.Fatalf("unexpected blocked reason: %+v", result.Root)
	}
}

func TestDeleteInstanceForceDeletes(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedInstanceData, fakeConnTracker{counts: map[string]int{"inst-1": 1}})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/instances/inst-1?force=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	getReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/instances/inst-1", nil)
	if err != nil {
		t.Fatalf("new get request: %v", err)
	}
	getReq.Header.Set("Authorization", "Bearer secret")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestDeleteStateBlockedWithoutRecurse(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedStateWithInstance, fakeConnTracker{})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/states/state-1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	var result deletion.DeleteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Root.Blocked != deletion.BlockHasDescendants {
		t.Fatalf("expected has_descendants, got %+v", result.Root)
	}
}

func TestDeleteStateRecurseBlockedByConnections(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedStateTree, fakeConnTracker{counts: map[string]int{"inst-child": 2}})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/states/state-root?recurse=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	var result deletion.DeleteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Root.Blocked != deletion.BlockBlockedDescendant {
		t.Fatalf("expected blocked_by_descendant, got %+v", result.Root)
	}
	if len(result.Root.Children) == 0 {
		t.Fatalf("expected child nodes")
	}
}

func TestDeleteStateRecurseForceDeletes(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedStateTree, fakeConnTracker{counts: map[string]int{"inst-child": 2}})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/states/state-root?recurse=true&force=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	getReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/states/state-root", nil)
	if err != nil {
		t.Fatalf("new get request: %v", err)
	}
	getReq.Header.Set("Authorization", "Bearer secret")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestDeleteStateDryRunReturns200(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedStateTree, fakeConnTracker{counts: map[string]int{"inst-child": 2}})
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/states/state-root?recurse=true&dry_run=true", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result deletion.DeleteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.DryRun || result.Outcome != deletion.OutcomeBlocked {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestListStatesIncludesParentID(t *testing.T) {
	server, cleanup := newDeleteTestServer(t, seedStateTree, fakeConnTracker{})
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/states/state-child", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entry store.StateEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if entry.ParentStateID == nil || *entry.ParentStateID != "state-root" {
		t.Fatalf("unexpected parent state id: %+v", entry.ParentStateID)
	}
}

func newDeleteTestServer(t *testing.T, seed func(*sql.DB) error, tracker conntrack.Tracker) (*httptest.Server, func()) {
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
	if err := seed(db); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	reg := registry.New(st)
	deleteMgr, err := deletion.NewManager(deletion.Options{
		Store: st,
		Conn:  tracker,
	})
	if err != nil {
		t.Fatalf("delete manager: %v", err)
	}
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   reg,
		Deletion:   deleteMgr,
	})
	server := httptest.NewServer(handler)

	cleanup := func() {
		server.Close()
		_ = db.Close()
		_ = st.Close()
	}
	return server, cleanup
}

func seedInstanceData(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO states (state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "state-1", nil, "state-1", "image-1", "psql", "args", now); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`, "inst-1", "state-1", "image-1", now, nil, nil)
	return err
}

func seedStateWithInstance(db *sql.DB) error {
	if err := seedInstanceData(db); err != nil {
		return err
	}
	return nil
}

func seedStateTree(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO states (state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "state-root", nil, "state-root", "image-1", "psql", "args", now); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO states (state_id, parent_state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "state-child", "state-root", "state-child", "image-1", "psql", "args", now); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`, "inst-child", "state-child", "image-1", now, nil, nil)
	return err
}
