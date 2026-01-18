package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/daemon"
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

func TestRunStatusLocalExplicitEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v2","instanceId":"inst","pid":2}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v2" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v3","instanceId":"inst","pid":3}`))
	}))
	defer server.Close()

	stateDir := t.TempDir()
	if err := daemon.WriteEngineState(filepath.Join(stateDir, "engine.json"), daemon.EngineState{
		Endpoint:   server.URL,
		AuthToken:  "token",
		InstanceID: "inst",
	}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: "",
		StateDir: stateDir,
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v3" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunStatus(context.Background(), StatusOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunStatusHealthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatalf("expected health error")
	}
	if result.Endpoint != server.URL {
		t.Fatalf("expected endpoint in result, got %+v", result)
	}
}

func TestRunStatusLocalAutostartDisabled(t *testing.T) {
	_, err := RunStatus(context.Background(), StatusOptions{
		Mode:      "local",
		Endpoint:  "",
		StateDir:  t.TempDir(),
		Autostart: false,
		Timeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "local engine is not running") {
		t.Fatalf("expected autostart error, got %v", err)
	}
}

func TestRunStatusRemoteVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok result")
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

func TestPrintStatusIncludesOptionalFields(t *testing.T) {
	result := StatusResult{
		OK:         false,
		Endpoint:   "http://localhost:2",
		Profile:    "remote",
		Mode:       "remote",
		Client:     "v1",
		Workspace:  "/tmp/sqlrs",
		Version:    "engine",
		InstanceID: "instance",
		PID:        7,
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "clientVersion: v1") {
		t.Fatalf("expected clientVersion, got %q", out)
	}
	if !strings.Contains(out, "workspace: /tmp/sqlrs") {
		t.Fatalf("expected workspace, got %q", out)
	}
	if !strings.Contains(out, "instanceId: instance") || !strings.Contains(out, "pid: 7") {
		t.Fatalf("expected instanceId and pid, got %q", out)
	}
}
