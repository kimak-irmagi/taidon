package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStatusRemoteJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"version":"v1","instanceId":"inst","pid":1}`)
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--mode=remote", "--endpoint", server.URL, "--output=json", "status"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "\"ok\":true") {
		t.Fatalf("unexpected output: %s", string(data))
	}
}

func TestRunLsCommandJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--mode=remote", "--endpoint", server.URL, "--output=json", "ls", "--names"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if _, ok := out["names"]; !ok {
		t.Fatalf("expected names in output, got %s", string(data))
	}
}

func TestRunRejectsInvalidOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"--output=bad", "status"})
	if err == nil || !strings.Contains(err.Error(), "invalid output") {
		t.Fatalf("expected invalid output error, got %v", err)
	}
}

func TestRunHelpOutputsUsage(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "Usage:") {
		t.Fatalf("expected usage output, got %q", string(data))
	}
}

func TestRunUnknownCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRunPrepareMissingKind(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"prepare"})
	if err == nil || !strings.Contains(err.Error(), "missing prepare kind") {
		t.Fatalf("expected missing prepare kind error, got %v", err)
	}
}

func TestRunPrepareUnknownKind(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"prepare:liquibase"})
	if err == nil || !strings.Contains(err.Error(), "unknown prepare kind") {
		t.Fatalf("expected unknown prepare kind error, got %v", err)
	}
}

func TestRunStatusRejectsArgs(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"status", "extra"})
	if err == nil || !strings.Contains(err.Error(), "status does not accept arguments") {
		t.Fatalf("expected status args error, got %v", err)
	}
}

func TestRunInvalidMode(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"--mode=weird", "status"})
	if err == nil || !strings.Contains(err.Error(), "invalid mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestRunStatusUnhealthy(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":false}`)
	}))
	defer server.Close()

	err := Run([]string{"--mode=remote", "--endpoint", server.URL, "status"})
	if err == nil || !strings.Contains(err.Error(), "service unhealthy") {
		t.Fatalf("expected unhealthy error, got %v", err)
	}
}

func TestRunRmCommandJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"dry_run":false,"outcome":"deleted","root":{"kind":"instance","id":"abc123456789abcd","connections":0}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--mode=remote", "--endpoint", server.URL, "--output=json", "rm", "abc12345"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "\"outcome\"") {
		t.Fatalf("unexpected output: %s", string(data))
	}
}

func TestRunPrepareCommand(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","result":{"dsn":"dsn","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-c select 1"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
	}()

	err = Run([]string{"--mode=remote", "--endpoint", server.URL, "prepare:psql", "--image", "image", "--", "-c", "select 1"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(data), "DSN=dsn") {
		t.Fatalf("unexpected output: %s", string(data))
	}
}

func TestRunInvalidOutputFromConfig(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	configDir := filepath.Join(temp, "config", "sqlrs")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("client:\n  output: bad\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := Run([]string{"status"})
	if err == nil || !strings.Contains(err.Error(), "invalid output") {
		t.Fatalf("expected invalid output, got %v", err)
	}
}

func TestRunProfileNotFound(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	configDir := filepath.Join(temp, "config", "sqlrs")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	configData := []byte("defaultProfile: missing\nprofiles: {}\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), configData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := Run([]string{"status"})
	if err == nil || !strings.Contains(err.Error(), "profile not found") {
		t.Fatalf("expected profile not found, got %v", err)
	}
}

func setTestDirs(t *testing.T, root string) {
	t.Helper()
	configDir := filepath.Join(root, "config")
	stateDir := filepath.Join(root, "state")
	cacheDir := filepath.Join(root, "cache")
	t.Setenv("APPDATA", configDir)
	t.Setenv("LOCALAPPDATA", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("SQLRSROOT", stateDir)
}
