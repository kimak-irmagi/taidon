package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunStatusRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v1","instanceId":"inst","pid":1}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v1" || result.InstanceID != "inst" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunStatus(context.Background(), StatusOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestPrintStatus(t *testing.T) {
	result := StatusResult{
		OK:       true,
		Endpoint: "http://localhost:1",
		Profile:  "local",
		Mode:     "remote",
		Version:  "v1",
		PID:      42,
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "status: ok") || !strings.Contains(out, "version: v1") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "workspace: (none)") {
		t.Fatalf("expected workspace placeholder, got %q", out)
	}
}
