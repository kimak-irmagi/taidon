package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHealthAddsSchemeAndPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := New(host, Options{Timeout: time.Second})
	_, err := cli.Health(context.Background())
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if gotPath != "/v1/health" {
		t.Fatalf("expected /v1/health path, got %q", gotPath)
	}
}

func TestHealthNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.Health(context.Background())
	if err == nil {
		t.Fatalf("expected health error")
	}
}

func TestListNamesAddsFilters(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	_, err := cli.ListNames(context.Background(), ListFilters{
		Instance: "i-1",
		State:    "s-1",
		Image:    "img-1",
	})
	if err != nil {
		t.Fatalf("list names failed: %v", err)
	}
	if gotPath != "/v1/names" {
		t.Fatalf("expected /v1/names path, got %q", gotPath)
	}
	if gotQuery.Get("instance") != "i-1" || gotQuery.Get("state") != "s-1" || gotQuery.Get("image") != "img-1" {
		t.Fatalf("unexpected query params: %v", gotQuery.Encode())
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
}

func TestListInstancesAddsIDPrefix(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListInstances(context.Background(), ListFilters{
		State:    "state-1",
		Image:    "image-1",
		IDPrefix: "deadbeef",
	})
	if err != nil {
		t.Fatalf("list instances failed: %v", err)
	}
	if gotQuery.Get("id_prefix") != "deadbeef" {
		t.Fatalf("expected id_prefix query, got %v", gotQuery.Encode())
	}
}

func TestGetInstanceFollowsRedirectWithAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/instances/dev":
			w.Header().Set("Location", "/v1/instances/abc")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/v1/instances/abc":
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	entry, found, err := cli.GetInstance(context.Background(), "dev")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !found || entry.InstanceID != "abc" {
		t.Fatalf("unexpected instance entry: %+v", entry)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization on redirected request, got %q", gotAuth)
	}
}

func TestListStatesAddsIDPrefix(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListStates(context.Background(), ListFilters{
		IDPrefix: "abc12345",
	})
	if err != nil {
		t.Fatalf("list states failed: %v", err)
	}
	if gotQuery.Get("id_prefix") != "abc12345" {
		t.Fatalf("expected id_prefix param, got %v", gotQuery.Encode())
	}
}

func TestCreatePrepareJob(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotRequest PrepareJobRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	accepted, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", "/abs/init.sql"},
	})
	if err != nil {
		t.Fatalf("create prepare job: %v", err)
	}
	if gotPath != "/v1/prepare-jobs" {
		t.Fatalf("expected /v1/prepare-jobs path, got %q", gotPath)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
	if gotRequest.ImageID != "image-1" || gotRequest.PrepareKind != "psql" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
	if accepted.JobID != "job-1" {
		t.Fatalf("unexpected accepted response: %+v", accepted)
	}
}

func TestCreatePrepareJobPlanOnly(t *testing.T) {
	var gotRequest PrepareJobRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err != nil {
		t.Fatalf("create prepare job: %v", err)
	}
	if !gotRequest.PlanOnly {
		t.Fatalf("expected plan_only true, got %+v", gotRequest)
	}
}

func TestGetNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, found, err := cli.GetName(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetName: %v", err)
	}
	if found {
		t.Fatalf("expected not found")
	}
}

func TestGetStateFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	entry, found, err := cli.GetState(context.Background(), "state")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if !found || entry.StateID != "state" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestDeleteInstanceAddsQuery(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"instance","id":"abc","connections":0}}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, status, err := cli.DeleteInstance(context.Background(), "abc", DeleteOptions{Force: true, DryRun: true, Recurse: true})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if gotQuery.Get("force") != "true" || gotQuery.Get("dry_run") != "true" {
		t.Fatalf("unexpected query: %v", gotQuery.Encode())
	}
	if gotQuery.Get("recurse") != "" {
		t.Fatalf("unexpected recurse for instance: %v", gotQuery.Encode())
	}
}

func TestDeleteStateAddsRecurse(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"state","id":"state"}}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, status, err := cli.DeleteState(context.Background(), "state", DeleteOptions{Recurse: true, DryRun: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if gotQuery.Get("recurse") != "true" {
		t.Fatalf("expected recurse=true, got %v", gotQuery.Encode())
	}
}

func TestGetPrepareJobNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, found, err := cli.GetPrepareJob(context.Background(), "job")
	if err != nil {
		t.Fatalf("GetPrepareJob: %v", err)
	}
	if found {
		t.Fatalf("expected not found")
	}
}

func TestGetPrepareJobParsesTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"img","prepare_args_normalized":"-c select 1","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"},{"task_id":"execute-0","type":"state_execute","input":{"kind":"image","id":"img"},"task_hash":"hash","output_state_id":"state","cached":false},{"task_id":"prepare-instance","type":"prepare_instance","input":{"kind":"state","id":"state"},"instance_mode":"ephemeral"}]}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	status, found, err := cli.GetPrepareJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetPrepareJob: %v", err)
	}
	if !found || !status.PlanOnly {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(status.Tasks) != 3 {
		t.Fatalf("expected tasks, got %d", len(status.Tasks))
	}
	if status.PrepareArgsNormalized == "" {
		t.Fatalf("expected normalized args")
	}
}

func TestCreatePrepareJobParsesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"message":"bad","details":"info"}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{PrepareKind: "psql", ImageID: "img"})
	if err == nil || !strings.Contains(err.Error(), "bad: info") {
		t.Fatalf("expected error message, got %v", err)
	}
}

func TestParseErrorResponseFallback(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader("not-json")),
	}
	err := parseErrorResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
}

func TestParseErrorResponseNil(t *testing.T) {
	err := parseErrorResponse(nil)
	if err == nil || !strings.Contains(err.Error(), "missing response") {
		t.Fatalf("expected missing response error, got %v", err)
	}
}

func TestHTTPStatusErrorMessage(t *testing.T) {
	err := (&HTTPStatusError{Status: "418 I'm a teapot"}).Error()
	if !strings.Contains(err, "418 I'm a teapot") {
		t.Fatalf("unexpected error message: %q", err)
	}
}
