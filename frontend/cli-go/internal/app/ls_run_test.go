package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/cli"
)

func TestRunLsWritesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`))
	}))
	defer server.Close()

	runOpts := cli.LsOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}

	var buf bytes.Buffer
	if err := runLs(&buf, runOpts, []string{"--names"}, "json"); err != nil {
		t.Fatalf("runLs: %v", err)
	}

	var out struct {
		Names []map[string]any `json:"names"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(out.Names) != 1 {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func TestRunLsInvalidPrefixMapsToExitError(t *testing.T) {
	runOpts := cli.LsOptions{
		Mode:     "remote",
		Endpoint: "http://127.0.0.1:1",
		Timeout:  time.Second,
	}
	err := runLs(&bytes.Buffer{}, runOpts, []string{"--instance", "abc"}, "human")
	if err == nil {
		t.Fatalf("expected error")
	}
	exitErr, ok := err.(*ExitError)
	if !ok || exitErr.Code != 2 || !strings.Contains(exitErr.Error(), "invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLsHelp(t *testing.T) {
	var buf bytes.Buffer
	runOpts := cli.LsOptions{}
	if err := runLs(&buf, runOpts, []string{"--help"}, "human"); err != nil {
		t.Fatalf("runLs: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", buf.String())
	}
}

func TestRunLsAmbiguousPrefixMapsToExitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"state_id":"deadbeef00000000","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0},{"state_id":"deadbeef11111111","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}]`))
	}))
	defer server.Close()

	runOpts := cli.LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
	}
	err := runLs(&bytes.Buffer{}, runOpts, []string{"--states", "--state", "deadbeef"}, "human")
	if err == nil {
		t.Fatalf("expected error")
	}
	exitErr, ok := err.(*ExitError)
	if !ok || exitErr.Code != 2 || !strings.Contains(exitErr.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLsWritesHuman(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`))
	}))
	defer server.Close()

	runOpts := cli.LsOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}

	var buf bytes.Buffer
	if err := runLs(&buf, runOpts, []string{"--names"}, "human"); err != nil {
		t.Fatalf("runLs: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Names") || !strings.Contains(out, "dev") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunLsJobsHumanForwardsSignatureWideLongAndQuietFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"job_id":"job-abcdef1234567890abcdef1234567890","status":"succeeded","prepare_kind":"psql","image_id":"postgres:17","resolved_image_id":"postgres@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","prepare_args_normalized":"-f /workspace/sql/prepare.sql -X -v ON_ERROR_STOP=1","signature":"sig-job-1","created_at":"2026-03-07T12:34:56.789Z","started_at":"2026-03-07T12:35:57.654Z","finished_at":"2026-03-07T12:36:58.321Z"}]`))
	}))
	defer server.Close()

	runOpts := cli.LsOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}

	var buf bytes.Buffer
	if err := runLs(&buf, runOpts, []string{"--jobs", "--signature", "--wide", "--long", "--no-header", "--quiet"}, "human"); err != nil {
		t.Fatalf("runLs: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"job-abcdef1234567890abcdef1234567890",
		"postgres@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"-f /workspace/sql/prepare.sql -X -v ON_ERROR_STOP=1",
		"sig-job-1",
		"2026-03-07T12:34:56Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
	if strings.Contains(out, "Jobs\n") || strings.Contains(out, "JOB_ID") {
		t.Fatalf("expected quiet no-header output, got %q", out)
	}
}

func TestRunLsReturnsUnhandledRunError(t *testing.T) {
	runOpts := cli.LsOptions{
		Mode:     "remote",
		Endpoint: "http://127.0.0.1:1",
		Timeout:  time.Second,
	}
	err := runLs(&bytes.Buffer{}, runOpts, []string{"--jobs"}, "human")
	if err == nil {
		t.Fatalf("expected error")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("expected raw error, got ExitError %v", err)
	}
}

func TestRunLsReturnsParseError(t *testing.T) {
	runOpts := cli.LsOptions{}
	err := runLs(&bytes.Buffer{}, runOpts, []string{"--unknown"}, "human")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
}
