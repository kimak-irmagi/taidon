package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunStatusLocalIncludesDeps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	withLocalDepsStub(t, func(ctx context.Context, opts LocalDepsOptions) (LocalDepsStatus, error) {
		return LocalDepsStatus{DockerReady: true, WSLReady: true, BtrfsReady: true}, nil
	})

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.DockerReady || !result.WSLReady || !result.BtrfsReady {
		t.Fatalf("unexpected deps result: %+v", result)
	}
}

func TestRunStatusLocalWarnsOnDeps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	withLocalDepsStub(t, func(ctx context.Context, opts LocalDepsOptions) (LocalDepsStatus, error) {
		return LocalDepsStatus{Warnings: []string{"btrfs not ready"}}, nil
	})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
		os.Stderr = oldStderr
	})

	_, err = RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}

	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if !strings.Contains(string(data), "btrfs not ready") {
		t.Fatalf("expected warning output, got %q", string(data))
	}
}

func TestPrintStatusIncludesDeps(t *testing.T) {
	result := StatusResult{
		OK:          true,
		Endpoint:    "http://localhost:1",
		Profile:     "local",
		Mode:        "local",
		DockerReady: true,
		WSLReady:    false,
		BtrfsReady:  false,
		Warnings:    []string{"wsl missing"},
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "container-runtime: ok") {
		t.Fatalf("expected container runtime status, got %q", out)
	}
	if !strings.Contains(out, "wsl: missing") || !strings.Contains(out, "btrfs: missing") {
		t.Fatalf("expected wsl/btrfs status, got %q", out)
	}
	if !strings.Contains(out, "warning: wsl missing") {
		t.Fatalf("expected warning line, got %q", out)
	}
}

func withLocalDepsStub(t *testing.T, fn func(context.Context, LocalDepsOptions) (LocalDepsStatus, error)) {
	t.Helper()
	prev := probeLocalDepsFn
	probeLocalDepsFn = fn
	t.Cleanup(func() {
		probeLocalDepsFn = prev
	})
}
