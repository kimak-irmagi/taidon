package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/store/sqlite"
)

func TestInstancesIDPrefixFilter(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/instances?id_prefix=BBBBBBBB", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entries []store.InstanceEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode instances: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(entries))
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.InstanceID, strings.ToLower("BBBBBBBB")) {
			t.Fatalf("unexpected instance id: %s", entry.InstanceID)
		}
	}
}

func TestInstancesIDPrefixInvalid(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/instances?id_prefix=bad!", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	var body prepare.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code == "" {
		t.Fatalf("expected error code")
	}
}

func TestInstancesIDPrefixTooShort(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/instances?id_prefix=abcd", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStatesIDPrefixFilter(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/states?id_prefix=aaaaaaaa", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entries []store.StateEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode states: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 states, got %d", len(entries))
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.StateID, "aaaaaaaa") {
			t.Fatalf("unexpected state id: %s", entry.StateID)
		}
	}
}

func TestStatesIDPrefixInvalid(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/states?id_prefix=deadbeeg", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	var body prepare.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Code == "" {
		t.Fatalf("expected error code")
	}
}

func TestStatesIDPrefixTooShort(t *testing.T) {
	server, cleanup := newPrefixTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/states?id_prefix=abc", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func newPrefixTestServer(t *testing.T) (*httptest.Server, func()) {
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
	if err := seedPrefixData(db); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	reg := registry.New(st)
	prep, err := prepare.NewManager(prepare.Options{
		Store:   st,
		Queue:   mustOpenQueue(t, dbPath),
		Version: "test",
		Async:   false,
	})
	if err != nil {
		t.Fatalf("prepare manager: %v", err)
	}
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   reg,
		Prepare:    prep,
	})
	server := httptest.NewServer(handler)

	cleanup := func() {
		server.Close()
		_ = db.Close()
		_ = st.Close()
	}
	return server, cleanup
}

func seedPrefixData(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stateA := strings.Repeat("a", 64)
	stateB := strings.Repeat("a", 63) + "b"
	if _, err := db.Exec(`INSERT INTO states (state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, stateA, stateA, "image-1", "psql", "a=1", now, int64(10), nil); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO states (state_id, state_fingerprint, image_id, prepare_kind, prepare_args_normalized, created_at, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, stateB, stateB, "image-2", "psql", "b=2", now, int64(20), nil); err != nil {
		return err
	}
	instanceA := strings.Repeat("b", 32)
	instanceB := strings.Repeat("b", 31) + "c"
	if _, err := db.Exec(`INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`, instanceA, stateA, "image-1", now, nil, nil); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO instances (instance_id, state_id, image_id, created_at, expires_at, status)
		VALUES (?, ?, ?, ?, ?, ?)`, instanceB, stateB, "image-2", now, nil, nil); err != nil {
		return err
	}
	return nil
}
