package app

import (
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

func TestRunRejectsInvalidOutput(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	err := Run([]string{"--output=bad", "status"})
	if err == nil || !strings.Contains(err.Error(), "invalid output") {
		t.Fatalf("expected invalid output error, got %v", err)
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
