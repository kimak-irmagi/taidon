package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunPrepareRemoteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunPrepare: %v", err)
	}
	if result.DSN != "dsn" || result.InstanceID != "inst" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunPrepareRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunPrepare(context.Background(), PrepareOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunPrepareFailedWithErrorDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"failed","error":{"message":"boom","details":"details"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunPrepare(context.Background(), PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		ImageID:  "image",
		PsqlArgs: []string{"-c", "select 1"},
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "boom: details") {
		t.Fatalf("expected error details, got %v", err)
	}
}
