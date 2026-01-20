package client

import (
	"context"
	"encoding/json"
	"errors"
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
		w.WriteHeader(http.StatusCreated)
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
		w.WriteHeader(http.StatusCreated)
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

func TestListPrepareJobsAddsFilter(t *testing.T) {
	var gotQuery url.Values
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListPrepareJobs(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListPrepareJobs: %v", err)
	}
	if gotPath != "/v1/prepare-jobs" {
		t.Fatalf("expected /v1/prepare-jobs, got %q", gotPath)
	}
	if gotQuery.Get("job") != "job-1" {
		t.Fatalf("expected job filter, got %v", gotQuery.Encode())
	}
}

func TestDeletePrepareJobAddsQuery(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"job","id":"job-1"}}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, status, err := cli.DeletePrepareJob(context.Background(), "job-1", DeleteOptions{Force: true, DryRun: true})
	if err != nil {
		t.Fatalf("DeletePrepareJob: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if gotQuery.Get("force") != "true" || gotQuery.Get("dry_run") != "true" {
		t.Fatalf("unexpected query: %v", gotQuery.Encode())
	}
}

func TestListTasksAddsFilter(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if gotQuery.Get("job") != "job-1" {
		t.Fatalf("expected job filter, got %v", gotQuery.Encode())
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

func TestNormalizeBaseURLEmpty(t *testing.T) {
	if normalizeBaseURL("") != "" {
		t.Fatalf("expected empty base url")
	}
}

func TestNormalizeBaseURLTrimsSlash(t *testing.T) {
	if normalizeBaseURL("https://example.com/") != "https://example.com" {
		t.Fatalf("expected trimmed base url")
	}
}

func TestAppendQueryEmpty(t *testing.T) {
	if appendQuery("/v1/names", url.Values{}) != "/v1/names" {
		t.Fatalf("expected unmodified path")
	}
}

func TestHealthInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `not-json`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.Health(context.Background())
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestGetNameServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, _, err := cli.GetName(context.Background(), "dev")
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
}

func TestDeletePrepareJobServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"message":"bad","details":"info"}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, _, err := cli.DeletePrepareJob(context.Background(), "job-1", DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "bad: info") {
		t.Fatalf("expected error message, got %v", err)
	}
}

func TestGetNameRedirectPreservesUserAgent(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/names/dev":
			w.Header().Set("Location", "/v1/names/real")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/v1/names/real":
			gotUserAgent = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"name":"real","image_id":"img","state_id":"state","status":"active"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, UserAgent: "TestUA"})
	_, found, err := cli.GetName(context.Background(), "dev")
	if err != nil {
		t.Fatalf("GetName: %v", err)
	}
	if !found || gotUserAgent != "TestUA" {
		t.Fatalf("expected user agent on redirect, got %q", gotUserAgent)
	}
}

func TestRedirectLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, _, err := cli.GetName(context.Background(), "dev")
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("expected redirect limit error, got %v", err)
	}
}

func TestDoRequestWithBodyNoContentType(t *testing.T) {
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	resp, err := cli.doRequestWithBody(context.Background(), http.MethodPost, "/custom", false, strings.NewReader("payload"), "")
	if err != nil {
		t.Fatalf("doRequestWithBody: %v", err)
	}
	_ = resp.Body.Close()
	if gotContentType != "" {
		t.Fatalf("expected empty content type, got %q", gotContentType)
	}
}

func TestParseErrorResponseEmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader("")),
	}
	err := parseErrorResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
}

func TestNewDefaultsTimeoutAndRedirectNoVia(t *testing.T) {
	cli := New("http://example.com", Options{})
	if cli.http.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout, got %v", cli.http.Timeout)
	}
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := cli.http.CheckRedirect(req, nil); err != nil {
		t.Fatalf("redirect check: %v", err)
	}
}

func TestListNamesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListNames(context.Background(), ListFilters{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestListInstancesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListInstances(context.Background(), ListFilters{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestListStatesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListStates(context.Background(), ListFilters{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreatePrepareJobRequestError(t *testing.T) {
	cli := New("http://127.0.0.1:1", Options{Timeout: 50 * time.Millisecond})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{PrepareKind: "psql", ImageID: "img"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreatePrepareJobDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `not-json`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{PrepareKind: "psql", ImageID: "img"})
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestListPrepareJobsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListPrepareJobs(context.Background(), "job-1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestListTasksError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ListTasks(context.Background(), "job-1")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHealthRequestError(t *testing.T) {
	cli := New("http://127.0.0.1:1", Options{Timeout: 50 * time.Millisecond})
	_, err := cli.Health(context.Background())
	if err == nil {
		t.Fatalf("expected request error")
	}
}

func TestDeleteStateRequestError(t *testing.T) {
	cli := New("http://127.0.0.1:1", Options{Timeout: 50 * time.Millisecond})
	_, _, err := cli.DeleteState(context.Background(), "state-1", DeleteOptions{})
	if err == nil {
		t.Fatalf("expected request error")
	}
}

func TestDeleteStateDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `not-json`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, _, err := cli.DeleteState(context.Background(), "state-1", DeleteOptions{})
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestDoRequestWithBodyUserAgent(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, UserAgent: "TestUA"})
	resp, err := cli.doRequestWithBody(context.Background(), http.MethodPost, "/custom", false, strings.NewReader("payload"), "application/json")
	if err != nil {
		t.Fatalf("doRequestWithBody: %v", err)
	}
	_ = resp.Body.Close()
	if gotUserAgent != "TestUA" {
		t.Fatalf("expected user agent, got %q", gotUserAgent)
	}
}

func TestParseErrorResponseMessageOnly(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader(`{"message":"bad"}`)),
	}
	err := parseErrorResponse(resp)
	if err == nil || err.Error() != "bad" {
		t.Fatalf("expected message error, got %v", err)
	}
}
