package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/run"
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
	"sqlrs/engine/internal/store/sqlite"
)

type fakeRunRuntime struct {
	output string
	err    error
}

func (f *fakeRunRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	return nil
}

func (f *fakeRunRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	return imageID, nil
}

func (f *fakeRunRuntime) Start(ctx context.Context, req engineRuntime.StartRequest) (engineRuntime.Instance, error) {
	return engineRuntime.Instance{}, nil
}

func (f *fakeRunRuntime) Stop(ctx context.Context, id string) error {
	return nil
}

func (f *fakeRunRuntime) Exec(ctx context.Context, id string, req engineRuntime.ExecRequest) (string, error) {
	if f.err != nil {
		return f.output, f.err
	}
	return f.output, nil
}

func (f *fakeRunRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	return nil
}

func newRunServer(t *testing.T, st store.Store, runtime engineRuntime.Runtime) *httptest.Server {
	t.Helper()
	reg := registry.New(st)
	runMgr, err := run.NewManager(run.Options{
		Registry: reg,
		Runtime:  runtime,
	})
	if err != nil {
		t.Fatalf("run manager: %v", err)
	}
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   reg,
		Run:        runMgr,
	})
	return httptest.NewServer(handler)
}

func createState(t *testing.T, st store.Store, stateID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "-c select 1",
		CreatedAt:             now,
		StateFingerprint:      "fp-" + stateID,
	}); err != nil {
		t.Fatalf("CreateState: %v", err)
	}
}

func createInstance(t *testing.T, st store.Store, instanceID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := store.InstanceStatusActive
	runtimeID := "container-1"
	createState(t, st, "state-1")
	if err := st.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    "state-1",
		ImageID:    "image-1",
		CreatedAt:  now,
		RuntimeID:  &runtimeID,
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
}

func TestRunEndpointRejectsUnknownKind(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"inst-1","kind":"unknown","args":[]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunEndpointMissingInstance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"missing","kind":"psql","args":[]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRunEndpointStreamsStartExit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	server := newRunServer(t, st, &fakeRunRuntime{output: "hello"})
	defer server.Close()

	body := `{"instance_ref":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","kind":"psql","args":["-c","select 1"]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(lines))
	}
	var types []string
	for _, line := range lines {
		var evt map[string]any
		if err := json.Unmarshal(line, &evt); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if value, ok := evt["type"].(string); ok {
			types = append(types, value)
		}
	}
	if !contains(types, "start") || !contains(types, "exit") {
		t.Fatalf("expected start and exit events, got %v", types)
	}
}

func TestRunEndpointStreamsSteps(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	server := newRunServer(t, st, &fakeRunRuntime{output: "hello"})
	defer server.Close()

	body := `{"instance_ref":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","kind":"psql","args":[],"steps":[{"args":["-c","select 1"]}]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	foundStdout := false
	for _, line := range lines {
		var evt map[string]any
		if err := json.Unmarshal(line, &evt); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if evt["type"] == "stdout" {
			foundStdout = true
		}
	}
	if !foundStdout {
		t.Fatalf("expected stdout event")
	}
}

func TestRunEndpointRejectsStepsWithArgs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "cccccccccccccccccccccccccccccccc")

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"cccccccccccccccccccccccccccccccc","kind":"psql","args":["-c","select 1"],"steps":[{"args":["-c","select 2"]}]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunEndpointRequiresAuth(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "dddddddddddddddddddddddddddddddd")

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"dddddddddddddddddddddddddddddddd","kind":"psql","args":[]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRunEndpointMethodNotAllowed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestRunEndpointInvalidJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader("{"))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunEndpointStepsWithStdin(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","kind":"psql","args":[],"stdin":"x","steps":[{"args":["-c","select 1"]}]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunEndpointConflictError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createState(t, st, "state-1")
	status := store.InstanceStatusActive
	if err := st.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: "ffffffffffffffffffffffffffffffff",
		StateID:    "state-1",
		ImageID:    "image-1",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		RuntimeID:  nil,
		Status:     &status,
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	server := newRunServer(t, st, &fakeRunRuntime{})
	defer server.Close()

	body := `{"instance_ref":"ffffffffffffffffffffffffffffffff","kind":"psql","args":[]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestRunEndpointInternalError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.db")
	st, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	createInstance(t, st, "abababababababababababababababab")

	server := newRunServer(t, st, &fakeRunRuntime{err: errors.New("boom")})
	defer server.Close()

	body := `{"instance_ref":"abababababababababababababababab","kind":"psql","args":[]}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestRunEndpointNilManager(t *testing.T) {
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   nil,
		Run:        nil,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/runs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
