package daemon

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectOrStartHealthyAfterLock(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	temp := t.TempDir()
	runDir := filepath.Join(temp, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}

	statePath := filepath.Join(temp, "engine.json")
	state := EngineState{Endpoint: server.URL}
	if err := WriteEngineState(statePath, state); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	result, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     "sqlrs-engine",
		RunDir:         runDir,
		StateDir:       temp,
		ClientTimeout:  time.Second,
		StartupTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("ConnectOrStart: %v", err)
	}
	if result.Endpoint != server.URL {
		t.Fatalf("unexpected endpoint: %q", result.Endpoint)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected multiple health checks, got %d", calls)
	}
}

func TestRunWSLCommandReturnsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	if _, err := runWSLCommand(ctx, "missing-distro", "true"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAttachVHDXToWSLRejectsEmpty(t *testing.T) {
	if err := attachVHDXToWSL(context.Background(), " ", false); err == nil {
		t.Fatalf("expected error for empty VHDX path")
	}
}

func TestAttachVHDXToWSLErrorIncludesOutput(t *testing.T) {
	prev := runHostCommandFn
	runHostCommandFn = func(ctx context.Context, args ...string) (string, error) {
		return "boom", errors.New("failed")
	}
	t.Cleanup(func() { runHostCommandFn = prev })

	err := attachVHDXToWSL(context.Background(), "C:\\temp\\store.vhdx", false)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected output in error, got %v", err)
	}
}

func TestIsExitStatus(t *testing.T) {
	err := exitCommand(7).Run()
	if err == nil {
		t.Fatalf("expected exit error")
	}
	if !isExitStatus(err, 7) {
		t.Fatalf("expected exit status 7")
	}
	if isExitStatus(err, 0) {
		t.Fatalf("expected non-matching exit status")
	}
}

func TestStartLogTailRejectsNilWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "engine.log")
	if err := os.WriteFile(logPath, []byte(""), 0o600); err != nil {
		t.Fatalf("create log: %v", err)
	}
	if _, err := startLogTail(logPath, nil); err == nil {
		t.Fatalf("expected error for nil writer")
	}
}

func TestStartLogTailSeekError(t *testing.T) {
	dir := t.TempDir()
	tailer, err := startLogTail(dir, &strings.Builder{})
	if err == nil {
		tailer.Stop()
		t.Skip("directory is seekable on this platform")
	}
}
