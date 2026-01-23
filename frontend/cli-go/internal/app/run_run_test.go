package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
)

func TestRunRunDefaultCommandWithArgs(t *testing.T) {
	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"start","ts":"2026-01-22T00:00:00Z","instance_id":"inst"}`+"\n")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	runOpts := cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}
	if err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, runOpts, "psql", []string{"--instance", "staging", "--", "-c", "select 1"}, "", ""); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	if gotRequest["instance_ref"] != "staging" || gotRequest["kind"] != "psql" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
	if _, ok := gotRequest["command"]; ok && gotRequest["command"] != nil {
		t.Fatalf("expected command to be omitted, got %+v", gotRequest["command"])
	}
	args, ok := gotRequest["args"].([]any)
	if !ok || len(args) != 0 {
		t.Fatalf("unexpected args: %+v", gotRequest["args"])
	}
	steps, ok := gotRequest["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("unexpected steps: %+v", gotRequest["steps"])
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected step: %+v", steps[0])
	}
	stepArgs, ok := step["args"].([]any)
	if !ok || len(stepArgs) != 2 || stepArgs[0] != "-c" || stepArgs[1] != "select 1" {
		t.Fatalf("unexpected step args: %+v", step["args"])
	}
}

func TestRunRunArgsWithoutCommand(t *testing.T) {
	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/runs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.WriteString(w, `{"type":"exit","ts":"2026-01-22T00:00:01Z","exit_code":0}`+"\n")
	}))
	defer server.Close()

	runOpts := cli.RunOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}
	if err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, runOpts, "psql", []string{"--instance", "staging", "-c", "select 1"}, "", ""); err != nil {
		t.Fatalf("runRun: %v", err)
	}
	args, ok := gotRequest["args"].([]any)
	if !ok || len(args) != 0 {
		t.Fatalf("unexpected args: %+v", gotRequest["args"])
	}
	steps, ok := gotRequest["steps"].([]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("unexpected steps: %+v", gotRequest["steps"])
	}
	step, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected step: %+v", steps[0])
	}
	stepArgs, ok := step["args"].([]any)
	if !ok || len(stepArgs) != 2 || stepArgs[0] != "-c" || stepArgs[1] != "select 1" {
		t.Fatalf("unexpected step args: %+v", step["args"])
	}
}

func TestRunRunRejectsPsqlConnectionArgs(t *testing.T) {
	runOpts := cli.RunOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, runOpts, "psql", []string{"--instance", "staging", "--", "-h", "127.0.0.1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "connection") {
		t.Fatalf("expected connection args error, got %v", err)
	}
}

func TestRunRunRejectsPgbenchConnectionArgs(t *testing.T) {
	runOpts := cli.RunOptions{Mode: "remote", Endpoint: "http://127.0.0.1:1"}
	err := runRun(&bytes.Buffer{}, &bytes.Buffer{}, runOpts, "pgbench", []string{"--instance", "staging", "--", "-h", "127.0.0.1"}, "", "")
	if err == nil || !strings.Contains(err.Error(), "connection") {
		t.Fatalf("expected connection args error, got %v", err)
	}
}
