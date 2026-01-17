package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

func TestPrintLsQuietSeparatesSections(t *testing.T) {
	result := LsResult{
		Names: &[]client.NameEntry{
			{
				Name:    "dev",
				ImageID: "image-1",
				StateID: "state-1",
				Status:  "active",
			},
		},
		Instances: &[]client.InstanceEntry{
			{
				InstanceID: "instance-1",
				ImageID:    "image-1",
				StateID:    "state-1",
				CreatedAt:  "2025-01-01T00:00:00Z",
				Status:     "active",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true})

	out := buf.String()
	if strings.Contains(out, "Names") || strings.Contains(out, "Instances") {
		t.Fatalf("unexpected section titles in quiet output: %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatalf("expected blank line between sections, got %q", out)
	}
}

func TestRunLsRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunLs(context.Background(), LsOptions{Mode: "remote", IncludeNames: true})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunLsLocalExplicitEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "local",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names: %+v", result.Names)
	}
}

func TestRunLsLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
		case "/v1/names":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	opts := LsOptions{
		Mode:         "local",
		Endpoint:     "",
		StateDir:     stateDir,
		Timeout:      time.Second,
		IncludeNames: true,
		Verbose:      true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names: %+v", result.Names)
	}
}

func TestRunLsUsesNameItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"dev","image_id":"img","state_id":"state","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/names/dev" {
		t.Fatalf("expected name item endpoint, got %q", gotPath)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names result: %+v", result.Names)
	}
}

func TestRunLsUsesInstanceItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances/dev" {
		t.Fatalf("expected instance item endpoint, got %q", gotPath)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}

func TestRunLsUsesInstancePrefixListEndpoint(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "abc12345",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances" {
		t.Fatalf("expected list instances endpoint, got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "id_prefix=abc12345") {
		t.Fatalf("expected id_prefix in query, got %q", gotQuery)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}

func TestRunLsStatePrefixNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeNames:     true,
		IncludeInstances: true,
		IncludeStates:    true,
		FilterState:      "deadbeef",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || result.Instances == nil || result.States == nil {
		t.Fatalf("expected all sections, got %+v", result)
	}
	if len(*result.Names) != 0 || len(*result.Instances) != 0 || len(*result.States) != 0 {
		t.Fatalf("expected empty results, got %+v", result)
	}
}

func TestRunLsListsAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/names":
			io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
		case "/v1/instances":
			io.WriteString(w, `[{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case "/v1/states":
			io.WriteString(w, `[{"state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeNames:     true,
		IncludeInstances: true,
		IncludeStates:    true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || result.Instances == nil || result.States == nil {
		t.Fatalf("expected all sections, got %+v", result)
	}
	if len(*result.Names) != 1 || len(*result.Instances) != 1 || len(*result.States) != 1 {
		t.Fatalf("unexpected results: %+v", result)
	}
}

func TestRunLsNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names/missing" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "missing",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 0 {
		t.Fatalf("expected empty names, got %+v", result.Names)
	}
}

func TestRunLsInstanceFromNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/dev" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 0 {
		t.Fatalf("expected empty instances, got %+v", result.Instances)
	}
}

func TestPrintLsStatesTable(t *testing.T) {
	size := int64(42)
	result := LsResult{
		States: &[]client.StateEntry{
			{
				StateID:     "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				ImageID:     "image-1",
				PrepareKind: "psql",
				PrepareArgs: "-c select 1",
				CreatedAt:   "2025-01-01T00:00:00Z",
				SizeBytes:   &size,
				RefCount:    2,
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{NoHeader: true, LongIDs: true})
	out := buf.String()
	if !strings.Contains(out, "psql") || !strings.Contains(out, "42") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestOptionalInt64(t *testing.T) {
	if optionalInt64(nil) != "" {
		t.Fatalf("expected empty string for nil")
	}
	value := int64(5)
	if optionalInt64(&value) != "5" {
		t.Fatalf("expected formatted value")
	}
}

func TestResolvedMatchValueOr(t *testing.T) {
	match := resolvedMatch{value: "state-1"}
	if match.valueOr("fallback") != "state-1" {
		t.Fatalf("expected match value")
	}
	empty := resolvedMatch{}
	if empty.valueOr("fallback") != "fallback" {
		t.Fatalf("expected fallback value")
	}
}

func TestNormalizeHexPrefix(t *testing.T) {
	if _, err := normalizeHexPrefix("abc"); err == nil {
		t.Fatalf("expected error for short prefix")
	}
	if _, err := normalizeHexPrefix("bad!1234"); err == nil {
		t.Fatalf("expected error for non-hex prefix")
	}
	value, err := normalizeHexPrefix("AbCdEf12")
	if err != nil {
		t.Fatalf("normalizeHexPrefix: %v", err)
	}
	if value != "abcdef12" {
		t.Fatalf("expected lowercase prefix, got %q", value)
	}
}
