package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
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

func TestParseConfigArgsErrors(t *testing.T) {
	if _, _, err := parseConfigArgs([]string{}); err == nil {
		t.Fatalf("expected error for missing command")
	}
	if _, _, err := parseConfigArgs([]string{"nope"}); err == nil {
		t.Fatalf("expected error for unknown command")
	}
	if _, _, err := parseConfigArgs([]string{"get"}); err == nil {
		t.Fatalf("expected error for missing path")
	}
	if _, _, err := parseConfigArgs([]string{"get", "-x"}); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
	if _, _, err := parseConfigArgs([]string{"get", "a", "b"}); err == nil {
		t.Fatalf("expected error for extra args")
	}
	if _, _, err := parseConfigArgs([]string{"set", "a"}); err == nil {
		t.Fatalf("expected error for missing value")
	}
	if _, _, err := parseConfigArgs([]string{"set", "a="}); err == nil {
		t.Fatalf("expected error for missing value")
	}
	if _, _, err := parseConfigArgs([]string{"rm"}); err == nil {
		t.Fatalf("expected error for missing path")
	}
	if _, _, err := parseConfigArgs([]string{"get", "—effective", "features.flag"}); err == nil || !strings.Contains(err.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash hint, got %v", err)
	}
}

func TestParseConfigArgsHelp(t *testing.T) {
	_, showHelp, err := parseConfigArgs([]string{"get", "--help"})
	if err != nil || !showHelp {
		t.Fatalf("expected help for get")
	}
	_, showHelp, err = parseConfigArgs([]string{"set", "-h"})
	if err != nil || !showHelp {
		t.Fatalf("expected help for set")
	}
	_, showHelp, err = parseConfigArgs([]string{"rm", "--help"})
	if err != nil || !showHelp {
		t.Fatalf("expected help for rm")
	}
	_, showHelp, err = parseConfigArgs([]string{"schema", "-h"})
	if err != nil || !showHelp {
		t.Fatalf("expected help for schema")
	}
}

func TestParseJSONValueErrors(t *testing.T) {
	if _, err := parseJSONValue("{"); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if _, err := parseJSONValue("true false"); err == nil {
		t.Fatalf("expected error for trailing data")
	}
}

func TestParseConfigArgsSetAssignment(t *testing.T) {
	cmd, showHelp, err := parseConfigArgs([]string{"set", "log.level=info"})
	if err != nil || showHelp {
		t.Fatalf("expected assignment parsing, got err=%v help=%v", err, showHelp)
	}
	if cmd.path != "log.level" || cmd.rawValue != "info" {
		t.Fatalf("unexpected assignment parse: %+v", cmd)
	}
}

func TestParseJSONValueAutoQuote(t *testing.T) {
	value, err := parseJSONValue("info")
	if err != nil {
		t.Fatalf("parseJSONValue: %v", err)
	}
	if value != "info" {
		t.Fatalf("expected string value, got %#v", value)
	}
	if _, err := parseJSONValue("with space"); err == nil {
		t.Fatalf("expected error for invalid JSON with spaces")
	}
}

func TestRunConfigSetJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	var out strings.Builder
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: server.URL}
	if err := runConfig(&out, runOpts, []string{"set", "features.flag", "true"}, "json"); err != nil {
		t.Fatalf("runConfig: %v", err)
	}
	if strings.Contains(out.String(), "config path=") {
		t.Fatalf("expected no diagnostics in json output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "\"path\":\"features.flag\"") {
		t.Fatalf("expected compact JSON, got %q", out.String())
	}
}

func TestRunConfigRemoveJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":null}`)
	}))
	defer server.Close()

	var out strings.Builder
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: server.URL}
	if err := runConfig(&out, runOpts, []string{"rm", "features.flag"}, "json"); err != nil {
		t.Fatalf("runConfig: %v", err)
	}
	if strings.Contains(out.String(), "config path=") {
		t.Fatalf("expected no diagnostics in json output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "\"path\":\"features.flag\"") {
		t.Fatalf("expected compact JSON, got %q", out.String())
	}
}

func TestRunConfigSchemaJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config/schema" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"type":"object"}`)
	}))
	defer server.Close()

	var out strings.Builder
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: server.URL}
	if err := runConfig(&out, runOpts, []string{"schema"}, "json"); err != nil {
		t.Fatalf("runConfig: %v", err)
	}
	if strings.Contains(out.String(), "config schema") {
		t.Fatalf("expected no diagnostics in json output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "\"type\":\"object\"") {
		t.Fatalf("expected compact JSON, got %q", out.String())
	}
}

func TestRunConfigSetInvalidJSONValue(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: "http://127.0.0.1"}
	if err := runConfig(io.Discard, runOpts, []string{"set", "features.flag", "{"}, "json"); err == nil {
		t.Fatalf("expected error for invalid JSON value")
	}
}

func TestRunConfigHelp(t *testing.T) {
	var out strings.Builder
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: "http://127.0.0.1"}
	if err := runConfig(&out, runOpts, []string{"get", "--help"}, "json"); err != nil {
		t.Fatalf("runConfig: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected usage output")
	}
}

func TestRunConfigSetBlankValue(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: "http://127.0.0.1"}
	if err := runConfig(io.Discard, runOpts, []string{"set", "features.flag", " "}, "json"); err == nil {
		t.Fatalf("expected error for blank value")
	}
}

func TestRunConfigGetError(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote"}
	if err := runConfig(io.Discard, runOpts, []string{"get", "features.flag"}, "json"); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigRemoveError(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote"}
	if err := runConfig(io.Discard, runOpts, []string{"rm", "features.flag"}, "json"); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigSchemaError(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote"}
	if err := runConfig(io.Discard, runOpts, []string{"schema"}, "json"); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigParseArgsError(t *testing.T) {
	runOpts := cli.ConfigOptions{Mode: "remote", Endpoint: "http://127.0.0.1"}
	if err := runConfig(io.Discard, runOpts, []string{}, "json"); err == nil {
		t.Fatalf("expected error for missing command")
	}
	if err := runConfig(io.Discard, runOpts, []string{"get"}, "json"); err == nil {
		t.Fatalf("expected error for missing path")
	}
}

func TestWritePrettyJSONError(t *testing.T) {
	if err := writePrettyJSON(io.Discard, map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatalf("expected error for non-marshalable value")
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
