package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompositeRunRejectsInstanceAfterPrepare(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	err := Run([]string{
		"--mode=remote",
		"--endpoint", server.URL,
		"prepare:psql", "--image", "image-1", "--", "-c", "select 1",
		"run:psql", "--instance", "staging", "--", "-c", "select 1",
	})
	if err == nil || !strings.Contains(err.Error(), "preceding prepare") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}
