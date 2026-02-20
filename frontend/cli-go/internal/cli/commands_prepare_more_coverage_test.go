package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
)

func TestWaitForPrepareStreamRequestErrorCoverage(t *testing.T) {
	cli := client.New("http://127.0.0.1:1", client.Options{Timeout: 50 * time.Millisecond})
	_, err := waitForPrepare(context.Background(), cli, "job-1", "http://127.0.0.1:1/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err == nil {
		t.Fatalf("expected stream request error")
	}
}

func TestWaitForPrepareEventsStatus5xxCoverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/prepare-jobs/job-1/events" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err == nil || !strings.Contains(err.Error(), "events stream returned status 500") {
		t.Fatalf("expected 5xx events status error, got %v", err)
	}
}

func TestWaitForPreparePartialRangeAdvanceCoverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("Content-Range", "events 5-5/6")
			w.WriteHeader(http.StatusPartialContent)
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"running"}`+"\n")
		case "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	status, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err != nil {
		t.Fatalf("waitForPrepare: %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestWaitForPrepareSkipsEmptyNDJSONLineCoverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, "\n")
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"running"}`+"\n")
		case "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	status, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err != nil {
		t.Fatalf("waitForPrepare: %v", err)
	}
	if status.Status != "succeeded" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
