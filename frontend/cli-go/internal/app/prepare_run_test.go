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
	"sqlrs/cli/internal/config"
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
