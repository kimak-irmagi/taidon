package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/cli"
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
