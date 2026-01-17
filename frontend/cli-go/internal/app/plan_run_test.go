package app

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

func TestRunPlanMissingImage(t *testing.T) {
	runOpts := cli.PrepareOptions{}
	err := runPlan(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--", "-c", "select 1"}, "json")
	if err == nil || !strings.Contains(err.Error(), "Missing base image id") {
		t.Fatalf("expected missing image error, got %v", err)
	}
}

func TestRunPlanHelp(t *testing.T) {
	var stdout bytes.Buffer
	runOpts := cli.PrepareOptions{}
	if err := runPlan(&stdout, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--help"}, "json"); err != nil {
		t.Fatalf("runPlan: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
}

func TestRunPlanRemoteRequiresEndpoint(t *testing.T) {
	runOpts := cli.PrepareOptions{Mode: "remote"}
	err := runPlan(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image", "--", "-c", "select 1"}, "json")
	if err == nil || !strings.Contains(err.Error(), "remote mode requires explicit endpoint") {
		t.Fatalf("expected remote endpoint error, got %v", err)
	}
}

func TestRunPlanNormalizeArgsError(t *testing.T) {
	runOpts := cli.PrepareOptions{}
	err := runPlan(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image", "--", "-f"}, "json")
	if err == nil || !strings.Contains(err.Error(), "Missing value for -f") {
		t.Fatalf("expected missing file value, got %v", err)
	}
}

func TestRunPlanVerboseImageSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-c select 1","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"},{"task_id":"execute-0","type":"state_execute","input":{"kind":"image","id":"image"},"task_hash":"hash","output_state_id":"state-1","cached":false},{"task_id":"prepare-instance","type":"prepare_instance","input":{"kind":"state","id":"state-1"},"instance_mode":"ephemeral"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var stderr bytes.Buffer
	runOpts := cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Verbose:  true,
	}
	if err := runPlan(&bytes.Buffer{}, &stderr, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image", "--", "-c", "select 1"}, "json"); err != nil {
		t.Fatalf("runPlan: %v", err)
	}
	if !strings.Contains(stderr.String(), "dbms.image=image (source: command line)") {
		t.Fatalf("expected image source in stderr, got %q", stderr.String())
	}
}
