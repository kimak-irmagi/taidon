package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConnectOrStartWithExplicitEndpoint(t *testing.T) {
	result, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint: "http://localhost:1234",
	})
	if err != nil {
		t.Fatalf("ConnectOrStart: %v", err)
	}
	if result.Endpoint != "http://localhost:1234" {
		t.Fatalf("unexpected endpoint: %q", result.Endpoint)
	}
}

func TestConnectOrStartAutostartDisabled(t *testing.T) {
	temp := t.TempDir()
	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:  "auto",
		Autostart: false,
		StateDir:  temp,
	})
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("expected not running error, got %v", err)
	}
}

func TestConnectOrStartMissingDaemonPath(t *testing.T) {
	temp := t.TempDir()
	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:  "auto",
		Autostart: true,
		RunDir:    filepath.Join(temp, "run"),
		StateDir:  temp,
	})
	if err == nil || !strings.Contains(err.Error(), "daemon path") {
		t.Fatalf("expected daemon path error, got %v", err)
	}
}

func TestConnectOrStartMissingRunDir(t *testing.T) {
	temp := t.TempDir()
	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:   "auto",
		Autostart:  true,
		DaemonPath: "sqlrs-engine",
		StateDir:   temp,
	})
	if err == nil || !strings.Contains(err.Error(), "runDir") {
		t.Fatalf("expected runDir error, got %v", err)
	}
}

func TestConnectOrStartReturnsHealthyState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
	}))
	defer server.Close()

	temp := t.TempDir()
	statePath := filepath.Join(temp, "engine.json")
	state := EngineState{
		Endpoint:   server.URL,
		InstanceID: "inst",
		AuthToken:  "token",
	}
	if err := WriteEngineState(statePath, state); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	result, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:      "auto",
		Autostart:     false,
		StateDir:      temp,
		ClientTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("ConnectOrStart: %v", err)
	}
	if result.Endpoint != server.URL || result.AuthToken != "token" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestConnectOrStartEnsureDirError(t *testing.T) {
	temp := t.TempDir()
	runDir := filepath.Join(temp, "run")
	if err := os.WriteFile(runDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:   "auto",
		Autostart:  true,
		DaemonPath: "sqlrs-engine",
		RunDir:     runDir,
		StateDir:   temp,
	})
	if err == nil {
		t.Fatalf("expected ensure dir error")
	}
}

func TestConnectOrStartStartError(t *testing.T) {
	temp := t.TempDir()
	runDir := filepath.Join(temp, "run")
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}

	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     filepath.Join(temp, "missing.exe"),
		RunDir:         runDir,
		StateDir:       temp,
		StartupTimeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected start error")
	}
}

func TestLoadHealthyState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
	}))
	defer server.Close()

	temp := t.TempDir()
	statePath := filepath.Join(temp, "engine.json")
	state := EngineState{
		Endpoint:   server.URL,
		InstanceID: "inst",
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	got, ok := loadHealthyState(context.Background(), statePath, 2*time.Second)
	if !ok || got.Endpoint != server.URL {
		t.Fatalf("expected healthy state, got %+v (ok=%v)", got, ok)
	}
}

func TestLogVerboseWrites(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		_ = w.Close()
		os.Stderr = old
	}()

	logVerbose(true, "hello %s", "world")
	_ = w.Close()
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestGatedWriterDisablesOutput(t *testing.T) {
	var buf bytes.Buffer
	writer := newGatedWriter(&buf)
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	writer.Disable()
	if _, err := writer.Write([]byte(" world")); err != nil {
		t.Fatalf("write disabled: %v", err)
	}
	if got := buf.String(); got != "hello" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestFormatEngineExitNil(t *testing.T) {
	err := formatEngineExit(nil)
	if err == nil || !strings.Contains(err.Error(), "exit code 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatEngineExitCode(t *testing.T) {
	cmd := exitCommand(7)
	execErr := cmd.Run()
	if execErr == nil {
		t.Fatalf("expected exit error")
	}
	err := formatEngineExit(execErr)
	if err == nil || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatEngineExitMessage(t *testing.T) {
	err := formatEngineExit(errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func exitCommand(code int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe", "/c", "exit", "/b", fmt.Sprintf("%d", code))
	}
	return exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
}
