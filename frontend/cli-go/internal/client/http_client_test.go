package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestResolveURL(t *testing.T) {
	cli := New("http://127.0.0.1:1234/", Options{})
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "  ", want: ""},
		{raw: "/v1/events", want: "http://127.0.0.1:1234/v1/events"},
		{raw: "v1/events", want: "http://127.0.0.1:1234/v1/events"},
		{raw: "http://example.com/x", want: "http://example.com/x"},
		{raw: "https://example.com", want: "https://example.com"},
	}
	for _, tc := range cases {
		if got := cli.resolveURL(tc.raw); got != tc.want {
			t.Fatalf("resolveURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestStreamPrepareEventsRejectsEmptyURL(t *testing.T) {
	cli := New("http://127.0.0.1:1234", Options{})
	if _, err := cli.StreamPrepareEvents(context.Background(), " ", ""); err == nil {
		t.Fatalf("expected error for empty url")
	}
}

func TestStreamPrepareEventsRelativeURL(t *testing.T) {
	var gotRange string
	var gotAuth string
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotRange = r.Header.Get("Range")
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "sqlrs-cli"})
	resp, err := cli.StreamPrepareEvents(context.Background(), "v1/prepare/events", "items=1-2")
	if err != nil {
		t.Fatalf("StreamPrepareEvents: %v", err)
	}
	resp.Body.Close()
	if gotRange != "items=1-2" {
		t.Fatalf("expected range header, got %q", gotRange)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if gotUA != "sqlrs-cli" {
		t.Fatalf("expected user-agent, got %q", gotUA)
	}
}

func TestStreamPrepareEventsAbsoluteURL(t *testing.T) {
	var sawRange bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Range") != "" {
			sawRange = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "sqlrs-cli"})
	resp, err := cli.StreamPrepareEvents(context.Background(), server.URL+"/v1/prepare/events", "")
	if err != nil {
		t.Fatalf("StreamPrepareEvents: %v", err)
	}
	resp.Body.Close()
	if sawRange {
		t.Fatalf("expected no range header")
	}
}

func TestListStates(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"state_id":"state-1","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1","created_at":"2026-01-01T00:00:00Z","refcount":1}]`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	states, err := cli.ListStates(context.Background(), ListFilters{Kind: "base", Image: "img", IDPrefix: "state"})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(states) != 1 || states[0].StateID != "state-1" {
		t.Fatalf("unexpected states: %+v", states)
	}
	if gotQuery.Get("kind") != "base" || gotQuery.Get("image") != "img" || gotQuery.Get("id_prefix") != "state" {
		t.Fatalf("unexpected query: %+v", gotQuery)
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

func TestGetStateFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/states/state-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"state_id":"state-1","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1","created_at":"2026-01-01T00:00:00Z","refcount":1}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	state, found, err := cli.GetState(context.Background(), "state-1")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if !found || state.StateID != "state-1" {
		t.Fatalf("unexpected result: %+v, found=%v", state, found)
	}
}

func TestGetStateNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, found, err := cli.GetState(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if found {
		t.Fatalf("expected not found")
	}
}

func TestDeleteInstanceOptions(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/instances/inst-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dry_run":true,"outcome":"deleted","root":{"kind":"instance","id":"inst-1"}}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	result, status, err := cli.DeleteInstance(context.Background(), "inst-1", DeleteOptions{Force: true, DryRun: true, Recurse: true})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if status != http.StatusOK || result.Outcome != "deleted" {
		t.Fatalf("unexpected delete result: %+v status=%d", result, status)
	}
	if gotQuery.Get("force") != "true" || gotQuery.Get("dry_run") != "true" {
		t.Fatalf("unexpected query: %+v", gotQuery)
	}
	if _, ok := gotQuery["recurse"]; ok {
		t.Fatalf("expected no recurse for DeleteInstance")
	}
}

func TestDeleteStateOptions(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/states/state-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dry_run":false,"outcome":"deleted","root":{"kind":"state","id":"state-1"}}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	result, status, err := cli.DeleteState(context.Background(), "state-1", DeleteOptions{Recurse: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if status != http.StatusOK || result.Root.Kind != "state" {
		t.Fatalf("unexpected delete result: %+v status=%d", result, status)
	}
	if gotQuery.Get("recurse") != "true" {
		t.Fatalf("expected recurse in query, got %+v", gotQuery)
	}
}

func TestCreatePrepareJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var req PrepareJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.PrepareKind != "psql" || req.ImageID != "image-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	resp, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("CreatePrepareJob: %v", err)
	}
	if resp.JobID != "job-1" || resp.StatusURL == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreatePrepareJobError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
	})
	if err == nil || err.Error() != "bad request" {
		t.Fatalf("expected bad request error, got %v", err)
	}
}

func TestCreatePrepareJobDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"job_id":`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
	})
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestCreatePrepareJobRequestError(t *testing.T) {
	cli := &Client{
		baseURL: "://",
		http:    &http.Client{Timeout: time.Second},
	}
	_, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
	})
	if err == nil {
		t.Fatalf("expected request error")
	}
}
