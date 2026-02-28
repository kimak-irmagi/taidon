package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
)

func TestParseWatchArgs(t *testing.T) {
	jobID, showHelp, err := parseWatchArgs([]string{"job-1"})
	if err != nil {
		t.Fatalf("parseWatchArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if jobID != "job-1" {
		t.Fatalf("expected job-1, got %q", jobID)
	}
}

func TestParseWatchArgsHelp(t *testing.T) {
	_, showHelp, err := parseWatchArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("parseWatchArgs: %v", err)
	}
	if !showHelp {
		t.Fatalf("expected showHelp=true")
	}
}

func TestParseWatchArgsRejectsExtraArgs(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"job-1", "job-2"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseWatchArgsRejectsUnknownOption(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"--bad"})
	if err == nil || !strings.Contains(err.Error(), "Unknown watch option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

func TestParseWatchArgsUnicodeDash(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"—help"})
	if err == nil || !strings.Contains(err.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash error, got %v", err)
	}
}

func TestRunWatchHelp(t *testing.T) {
	var out bytes.Buffer
	err := runWatch(&out, cli.PrepareOptions{}, []string{"--help"})
	if err != nil {
		t.Fatalf("runWatch: %v", err)
	}
	if !strings.Contains(out.String(), "sqlrs watch <job-id>") {
		t.Fatalf("expected watch usage output, got %q", out.String())
	}
}

func TestRunWatchSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare-jobs/job-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runWatch(&out, cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}, []string{"job-1"})
	if err != nil {
		t.Fatalf("runWatch: %v", err)
	}
}

func TestRunWatchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	err := runWatch(&bytes.Buffer{}, cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}, []string{"job-1"})
	if err == nil || !strings.Contains(err.Error(), "prepare job not found") {
		t.Fatalf("expected watch error, got %v", err)
	}
}

func TestPrepareAcceptedFromDetached(t *testing.T) {
	accepted := prepareAcceptedFromDetached("job-42")
	if accepted.JobID != "job-42" {
		t.Fatalf("unexpected job id: %+v", accepted)
	}
	if accepted.StatusURL != "/v1/prepare-jobs/job-42" {
		t.Fatalf("unexpected status url: %+v", accepted)
	}
	if accepted.EventsURL != "/v1/prepare-jobs/job-42/events" {
		t.Fatalf("unexpected events url: %+v", accepted)
	}
}
