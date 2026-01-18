package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
	"sqlrs/cli/internal/paths"
)

func TestRunPrepareRemote(t *testing.T) {
	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"postgres://sqlrs@local/instance/abc","instance_id":"abc","state_id":"state","image_id":"image-1","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	runOpts := cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}

	cfg := config.LoadedConfig{}
	var stdout bytes.Buffer
	if err := runPrepare(&stdout, io.Discard, runOpts, cfg, t.TempDir(), []string{"--image", "image-1", "--", "-c", "select 1"}); err != nil {
		t.Fatalf("runPrepare: %v", err)
	}
	if !strings.Contains(stdout.String(), "DSN=postgres://sqlrs@local/instance/abc") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
	if gotRequest["image_id"] != "image-1" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
}

func TestRunPrepareMissingImage(t *testing.T) {
	runOpts := cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: "http://127.0.0.1:1",
		Timeout:  time.Second,
	}

	err := runPrepare(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--", "-c", "select 1"})
	if err == nil {
		t.Fatalf("expected error")
	}
	exitErr, ok := err.(*ExitError)
	if !ok || exitErr.Code != 2 {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPrepareVerboseImageSource(t *testing.T) {
	var gotRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"postgres://sqlrs@local/instance/abc","instance_id":"abc","state_id":"state","image_id":"image-1","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	temp := t.TempDir()
	configDir := filepath.Join(temp, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("dbms:\n  image: image-1\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	runOpts := cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Verbose:  true,
	}

	cfg := config.LoadedConfig{Paths: paths.Dirs{ConfigDir: configDir}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runPrepare(&stdout, &stderr, runOpts, cfg, temp, []string{"--", "-c", "select 1"}); err != nil {
		t.Fatalf("runPrepare: %v", err)
	}
	if !strings.Contains(stderr.String(), "dbms.image=image-1") {
		t.Fatalf("expected image source, got %q", stderr.String())
	}
	if gotRequest["image_id"] != "image-1" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
}

func TestRunPrepareHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runPrepare(&stdout, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), []string{"--help"}); err != nil {
		t.Fatalf("runPrepare: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
}

func TestRunPrepareRemoteRequiresEndpoint(t *testing.T) {
	runOpts := cli.PrepareOptions{
		Mode: "remote",
	}
	err := runPrepare(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image-1", "--", "-c", "select 1"})
	if err == nil || !strings.Contains(err.Error(), "remote mode requires explicit endpoint") {
		t.Fatalf("expected remote endpoint error, got %v", err)
	}
}

func TestRunPrepareNormalizeArgsError(t *testing.T) {
	err := runPrepare(&bytes.Buffer{}, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image-1", "--", "-f"})
	if err == nil || !strings.Contains(err.Error(), "Missing value for -f") {
		t.Fatalf("expected missing file value, got %v", err)
	}
}

func TestRunPrepareResolveImageError(t *testing.T) {
	temp := t.TempDir()
	projectDir := filepath.Join(temp, "project", ".sqlrs")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	projectPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(projectPath, []byte("dbms: ["), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	cfg := config.LoadedConfig{ProjectConfigPath: projectPath}

	err := runPrepare(&bytes.Buffer{}, io.Discard, cli.PrepareOptions{}, cfg, temp, []string{"--", "-c", "select 1"})
	if err == nil {
		t.Fatalf("expected resolve image error")
	}
}

func TestRunPrepareRemoteServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	runOpts := cli.PrepareOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	}
	err := runPrepare(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--image", "image-1", "--", "-c", "select 1"})
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected server error, got %v", err)
	}
}
