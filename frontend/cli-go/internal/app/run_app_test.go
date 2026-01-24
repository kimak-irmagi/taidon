package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCommandRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	err := Run([]string{
		"--mode=remote",
		"--endpoint", server.URL,
		"run:psql", "--instance", "staging", "--", "-c", "select 1",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunCommandPgbenchRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	err := Run([]string{
		"--mode=remote",
		"--endpoint", server.URL,
		"run:pgbench", "--instance", "staging", "--", "-c", "10",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunCommandUnknownRunKind(t *testing.T) {
	err := Run([]string{
		"--mode=remote",
		"--endpoint", "http://127.0.0.1:1",
		"run:unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown run kind") {
		t.Fatalf("expected unknown run kind, got %v", err)
	}
}

func TestRunCommandMissingRunKind(t *testing.T) {
	err := Run([]string{
		"--mode=remote",
		"--endpoint", "http://127.0.0.1:1",
		"run",
	})
	if err == nil || !strings.Contains(err.Error(), "missing run kind") {
		t.Fatalf("expected missing run kind, got %v", err)
	}
}
