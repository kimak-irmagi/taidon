package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/prepare"
	"sqlrs/engine/internal/registry"
	"sqlrs/engine/internal/store/sqlite"
)

func TestPrepareJobCancelMethodNotAllowed(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/job-1/cancel", nil)
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

func TestPrepareJobCancelInvalidPath(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs/job/extra/cancel", nil)
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

func TestPrepareJobCancelNotFound(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs/missing/cancel", nil)
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

func TestPrepareJobCancelAlreadyTerminalReturnsOK(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	jobID := submitPlanOnlyJob(t, server.URL, "secret")
	if err := waitForPrepareCompletion(server.URL, "/v1/prepare-jobs/"+jobID, "secret"); err != nil {
		t.Fatalf("wait for completion: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs/"+jobID+"/cancel", nil)
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
	var status prepare.Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Status != prepare.StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
}

func TestPrepareJobCancelRunningReturnsAccepted(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	blocker := newBlockingStore(st)
	prep := newPrepareManager(t, blocker, mustOpenQueue(t, filepath.Join(dir, "state.db")), func(opts *prepare.Options) {
		opts.Async = true
	})
	handler := NewHandler(Options{
		Version:    "test",
		InstanceID: "instance",
		AuthToken:  "secret",
		Registry:   registry.New(st),
		Prepare:    prep,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	jobID := submitPrepareJob(t, server.URL, "secret", false)
	waitForChannel(t, blocker.started)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/prepare-jobs/"+jobID+"/cancel", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		resp.Body.Close()
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	close(blocker.release)
	if err := waitForPrepareCompletion(server.URL, "/v1/prepare-jobs/"+jobID, "secret"); err != nil {
		t.Fatalf("wait for completion: %v", err)
	}

	statusReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/prepare-jobs/"+jobID, nil)
	if err != nil {
		t.Fatalf("new status request: %v", err)
	}
	statusReq.Header.Set("Authorization", "Bearer secret")
	statusResp, err := http.DefaultClient.Do(statusReq)
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", statusResp.StatusCode)
	}
	var status prepare.Status
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Status != prepare.StatusFailed {
		t.Fatalf("expected failed status, got %q", status.Status)
	}
	if status.Error == nil || status.Error.Code != "cancelled" {
		t.Fatalf("expected cancelled error payload, got %+v", status.Error)
	}
}
