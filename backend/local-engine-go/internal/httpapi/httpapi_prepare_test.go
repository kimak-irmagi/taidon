package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store/sqlite"
)

func TestPrepareJobsRequireAuth(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestPrepareJobsMethodNotAllowed(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestPrepareJobsRejectsInvalidKind(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	payload := `{"prepare_kind":"liquibase","image_id":"img","psql_args":[]}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", strings.NewReader(payload))
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

func TestPrepareJobsInternalError(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	reg := registry.New(st)
	prep, err := prepare.NewManager(prepare.Options{
		Store: st,
		IDGen: func() (string, error) { return "", errors.New("boom") },
		Async: false,
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
	defer server.Close()

	reqBody := prepare.Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestPrepareJobStatusMethodNotAllowed(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs/job-1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestPrepareJobStatusNotFoundPath(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/job/extra", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPrepareJobEventsInvalidPath(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/job/extra/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestPrepareJobsCreateAndEvents(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	temp := t.TempDir()
	filePath := filepath.Join(temp, "init.sql")
	if err := os.WriteFile(filePath, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reqBody := prepare.Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", filePath},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatalf("expected Location header")
	}
	var accepted prepare.Accepted
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode accepted: %v", err)
	}
	if accepted.JobID == "" {
		t.Fatalf("expected job_id")
	}

	status, err := pollPrepareStatus(server.URL, location, "secret")
	if err != nil {
		t.Fatalf("poll status: %v", err)
	}
	if status.Status != prepare.StatusSucceeded {
		t.Fatalf("expected succeeded, got %q", status.Status)
	}
	if status.Result == nil || status.Result.InstanceID == "" || status.Result.DSN == "" {
		t.Fatalf("expected result with instance and dsn, got %+v", status.Result)
	}

	eventURL := server.URL + location + "/events"
	eventReq, err := http.NewRequest(http.MethodGet, eventURL, nil)
	if err != nil {
		t.Fatalf("new event request: %v", err)
	}
	eventReq.Header.Set("Authorization", "Bearer secret")
	eventResp, err := http.DefaultClient.Do(eventReq)
	if err != nil {
		t.Fatalf("event request: %v", err)
	}
	defer eventResp.Body.Close()

	if eventResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 events, got %d", eventResp.StatusCode)
	}
	if !strings.HasPrefix(eventResp.Header.Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("expected ndjson content type, got %q", eventResp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(eventResp.Body)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected event lines")
	}
	foundResult := false
	for _, line := range lines {
		var event prepare.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if event.Type == "result" {
			foundResult = true
		}
	}
	if !foundResult {
		t.Fatalf("expected result event")
	}
}

func TestPrepareJobsPlanOnly(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	reqBody := prepare.Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatalf("expected Location header")
	}

	status, err := pollPrepareStatus(server.URL, location, "secret")
	if err != nil {
		t.Fatalf("poll status: %v", err)
	}
	if status.Status != prepare.StatusSucceeded || !status.PlanOnly {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Result != nil {
		t.Fatalf("expected no result, got %+v", status.Result)
	}
	if status.PrepareArgsNormalized == "" {
		t.Fatalf("expected normalized args")
	}
	if len(status.Tasks) != 3 {
		t.Fatalf("expected tasks, got %d", len(status.Tasks))
	}
	if status.Tasks[1].Cached == nil {
		t.Fatalf("expected cached flag")
	}
}

func TestPrepareEventsWithoutFlusher(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example/v1/prepare-jobs/job/events", nil)
	writer := &noFlushWriter{}

	streamPrepareEvents(writer, req, nil, "job")
	if writer.code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", writer.code)
	}
}

type noFlushWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *noFlushWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *noFlushWriter) WriteHeader(status int) {
	w.code = status
}

func pollPrepareStatus(baseURL, location, token string) (prepare.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var last prepare.Status
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+location, nil)
		if err != nil {
			return prepare.Status{}, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return prepare.Status{}, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return prepare.Status{}, &httpStatusError{StatusCode: resp.StatusCode}
		}
		if err := json.NewDecoder(resp.Body).Decode(&last); err != nil {
			resp.Body.Close()
			return prepare.Status{}, err
		}
		resp.Body.Close()
		if last.Status == prepare.StatusSucceeded || last.Status == prepare.StatusFailed {
			return last, nil
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type httpStatusError struct {
	StatusCode int
}

func (e *httpStatusError) Error() string {
	return "unexpected status"
}
