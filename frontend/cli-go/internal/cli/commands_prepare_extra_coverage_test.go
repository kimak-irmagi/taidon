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

func TestRunPrepareEventsURLMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"job-1","events_url":"   "}`)
	}))
	t.Cleanup(server.Close)

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "img",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "events url missing") {
		t.Fatalf("expected events url error, got %v", err)
	}
}

func TestRunPlanEventsURLMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"job-1","events_url":"   "}`)
	}))
	t.Cleanup(server.Close)

	_, err := RunPlan(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "img",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "events url missing") {
		t.Fatalf("expected events url error, got %v", err)
	}
}

func TestWaitForPrepareMissingEventsURL(t *testing.T) {
	_, err := waitForPrepare(context.Background(), client.New("http://example.com", client.Options{}), "job-1", "   ", io.Discard, false)
	if err == nil || !strings.Contains(err.Error(), "events url missing") {
		t.Fatalf("expected missing events url error, got %v", err)
	}
}

func TestWaitForPrepareInvalidContentRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/prepare-jobs/job-1/events" {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("Content-Range", "broken")
			w.WriteHeader(http.StatusPartialContent)
			io.WriteString(w, "{}\n")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err == nil || !strings.Contains(err.Error(), "content range") {
		t.Fatalf("expected content-range error, got %v", err)
	}
}

func TestWaitForPrepareGetPrepareJobError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/prepare-jobs/job-1/events":
			writeEventStream(w, []client.PrepareJobEvent{statusEvent("running")})
		case "/v1/prepare-jobs/job-1":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err == nil {
		t.Fatalf("expected job status error")
	}
}

func TestWaitForPrepareInvalidJSONEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs/job-1/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, "{bad\n")
	}))
	t.Cleanup(server.Close)

	cli := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := waitForPrepare(context.Background(), cli, "job-1", server.URL+"/v1/prepare-jobs/job-1/events", io.Discard, false)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
}

func TestParseEventsContentRangeErrors(t *testing.T) {
	cases := []string{
		"",
		"bytes 1-2/3",
		"events 1/3",
		"events a-b/3",
	}
	for _, in := range cases {
		if _, err := parseEventsContentRange(in); err == nil {
			t.Fatalf("expected parse error for %q", in)
		}
	}
}

func TestFormatPrepareEventAdditionalBranches(t *testing.T) {
	if got := formatPrepareEvent(client.PrepareJobEvent{Type: "task"}); got != "prepare task" {
		t.Fatalf("unexpected bare task event: %q", got)
	}
	if got := formatPrepareEvent(client.PrepareJobEvent{Type: "note", Error: &client.ErrorResponse{Details: "only-details"}}); got != "prepare note: only-details" {
		t.Fatalf("unexpected note event: %q", got)
	}
}
