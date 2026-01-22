package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRunConfigGetHuman(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	output := captureStdout(t, func() error {
		return Run([]string{"--mode=remote", "--endpoint", server.URL, "config", "get", "features.flag"})
	})

	if !strings.Contains(output, "config path=features.flag") {
		t.Fatalf("expected diagnostic output, got %q", output)
	}
	if !strings.Contains(output, "\n  \"path\": \"features.flag\"") {
		t.Fatalf("expected pretty JSON, got %q", output)
	}
}

func TestRunConfigGetJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	output := captureStdout(t, func() error {
		return Run([]string{"--mode=remote", "--endpoint", server.URL, "--output=json", "config", "get", "features.flag"})
	})

	if strings.Contains(output, "config path=") {
		t.Fatalf("expected no diagnostics in json output, got %q", output)
	}
	if !strings.Contains(output, "\"path\":\"features.flag\"") {
		t.Fatalf("expected compact JSON, got %q", output)
	}
}

func TestRunConfigSet(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(data), "\"path\":\"features.flag\"") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	output := captureStdout(t, func() error {
		return Run([]string{"--mode=remote", "--endpoint", server.URL, "config", "set", "features.flag", "true"})
	})

	if !strings.Contains(output, "config path=features.flag") {
		t.Fatalf("expected diagnostic output, got %q", output)
	}
}

func TestRunConfigRm(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("path") != "features.flag" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":null}`)
	}))
	defer server.Close()

	output := captureStdout(t, func() error {
		return Run([]string{"--mode=remote", "--endpoint", server.URL, "config", "rm", "features.flag"})
	})

	if !strings.Contains(output, "config path=features.flag") {
		t.Fatalf("expected diagnostic output, got %q", output)
	}
}

func TestRunConfigSchema(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config/schema" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"type":"object"}`)
	}))
	defer server.Close()

	output := captureStdout(t, func() error {
		return Run([]string{"--mode=remote", "--endpoint", server.URL, "config", "schema"})
	})

	if !strings.Contains(output, "config schema") {
		t.Fatalf("expected diagnostic output, got %q", output)
	}
	if !strings.Contains(output, "\n  \"type\": \"object\"") {
		t.Fatalf("expected pretty JSON, got %q", output)
	}
}

func TestRunConfigEffectiveFlag(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	var gotEffective bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotEffective = r.URL.Query().Get("effective") == "true"
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	if err := Run([]string{"--mode=remote", "--endpoint", server.URL, "config", "get", "features.flag", "--effective"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !gotEffective {
		t.Fatalf("expected effective=true query")
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
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

	if err := fn(); err != nil {
		_ = w.Close()
		t.Fatalf("Run: %v", err)
	}
	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}
