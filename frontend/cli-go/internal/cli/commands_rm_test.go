package cli

import (
	"bytes"
	"context"
	"errors"
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

func TestRunRmInstanceDelete(t *testing.T) {
	var gotDeletePath string
	var gotDeleteQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			gotDeletePath = r.URL.Path
			gotDeleteQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"instance","id":"abc123456789abcd","connections":0}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := RmOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
		DryRun:   true,
		Force:    true,
	}
	result, err := RunRm(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	if result.NoMatch || result.Delete == nil {
		t.Fatalf("expected delete result, got %+v", result)
	}
	if gotDeletePath != "/v1/instances/abc123456789abcd" {
		t.Fatalf("unexpected delete path: %q", gotDeletePath)
	}
	if !strings.Contains(gotDeleteQuery, "dry_run=true") || !strings.Contains(gotDeleteQuery, "force=true") {
		t.Fatalf("unexpected delete query: %q", gotDeleteQuery)
	}
}

func TestRunRmLocalExplicitEndpoint(t *testing.T) {
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
			io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"instance","id":"abc123456789abcd","connections":0}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := RmOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
		DryRun:   true,
	}
	result, err := RunRm(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	if result.Delete == nil || result.Delete.Outcome != "would_delete" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunRmLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/instances/"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"dry_run":true,"outcome":"would_delete","root":{"kind":"instance","id":"abc123456789abcd","connections":0}}`)
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

	opts := RmOptions{
		Mode:     "local",
		Endpoint: "",
		StateDir: stateDir,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
		DryRun:   true,
		Verbose:  true,
	}
	result, err := RunRm(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	if result.Delete == nil || result.Delete.Outcome != "would_delete" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunRmAmbiguousResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"state_id":"def","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := RmOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
	}
	_, err := RunRm(context.Background(), opts)
	if err == nil {
		t.Fatalf("expected ambiguous error")
	}
	var ambErr *AmbiguousResourceError
	if !errors.As(err, &ambErr) {
		t.Fatalf("expected AmbiguousResourceError, got %v", err)
	}
}

func TestRunRmRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunRm(context.Background(), RmOptions{Mode: "remote", IDPrefix: "deadbeef"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunRmAmbiguousInstancePrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"},{"instance_id":"def","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := RunRm(context.Background(), RmOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
	})
	var ambErr *AmbiguousPrefixError
	if !errors.As(err, &ambErr) || ambErr.Kind != "instance" {
		t.Fatalf("expected AmbiguousPrefixError for instance, got %v", err)
	}
}

func TestRunRmNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	opts := RmOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		IDPrefix: "abc12345",
	}
	result, err := RunRm(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunRm: %v", err)
	}
	if !result.NoMatch {
		t.Fatalf("expected NoMatch, got %+v", result)
	}
}

func TestPrintRmShowsBlocked(t *testing.T) {
	result := client.DeleteResult{
		DryRun:  false,
		Outcome: "blocked",
		Root: client.DeleteNode{
			Kind:    "state",
			ID:      "abc",
			Blocked: "blocked_by_descendant",
			Children: []client.DeleteNode{
				{
					Kind:        "instance",
					ID:          "def",
					Blocked:     "active_connections",
					Connections: intPtr(2),
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintRm(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "blocked (blocked_by_descendant)") {
		t.Fatalf("expected blocked output, got %q", out)
	}
	if !strings.Contains(out, "connections=2") {
		t.Fatalf("expected connections output, got %q", out)
	}
}

func TestNodeActionVariants(t *testing.T) {
	blocked := nodeAction(client.DeleteNode{Blocked: "active_connections"}, client.DeleteResult{})
	if !strings.Contains(blocked, "blocked (active_connections)") {
		t.Fatalf("unexpected blocked action: %q", blocked)
	}
	wouldDelete := nodeAction(client.DeleteNode{}, client.DeleteResult{DryRun: true})
	if wouldDelete != "would delete" {
		t.Fatalf("unexpected dry-run action: %q", wouldDelete)
	}
	deleted := nodeAction(client.DeleteNode{}, client.DeleteResult{Outcome: "deleted"})
	if deleted != "deleted" {
		t.Fatalf("unexpected deleted action: %q", deleted)
	}

	node := client.DeleteNode{}
	if nodeConnections(node) != 0 {
		t.Fatalf("expected zero connections")
	}
	node.Connections = intPtr(3)
	if nodeConnections(node) != 3 {
		t.Fatalf("expected connections=3")
	}
}

func intPtr(value int) *int {
	return &value
}

func TestAmbiguousResourceErrorMessage(t *testing.T) {
	err := &AmbiguousResourceError{Prefix: "deadbeef"}
	if err.Error() == "" {
		t.Fatalf("expected error message")
	}
}
