package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseLsCacheDetailsWithStates(t *testing.T) {
	opts, showHelp, err := parseLsFlags([]string{"--states", "--cache-details"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp false")
	}
	if !opts.IncludeStates || !opts.CacheDetails {
		t.Fatalf("expected states+cache-details, got %+v", opts)
	}
}

func TestParseLsCacheDetailsWithAll(t *testing.T) {
	opts, _, err := parseLsFlags([]string{"--all", "--cache-details"})
	if err != nil {
		t.Fatalf("parseLsFlags: %v", err)
	}
	if !opts.IncludeStates || !opts.CacheDetails {
		t.Fatalf("expected --all to allow cache details, got %+v", opts)
	}
}

func TestParseLsCacheDetailsRequiresStatesOrAll(t *testing.T) {
	_, _, err := parseLsFlags([]string{"--cache-details"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
	if !strings.Contains(exitErr.Error(), "--states") || !strings.Contains(exitErr.Error(), "--all") {
		t.Fatalf("expected selector hint, got %v", exitErr)
	}
}

func TestRunLsStatesCacheDetailsJSON(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"state_id":"state-1","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"-c select 1","created_at":"2026-01-01T00:00:00Z","size_bytes":42,"last_used_at":"2026-03-09T12:00:00Z","use_count":7,"min_retention_until":"2026-03-09T12:10:00Z","refcount":1}]`)
	}))
	defer server.Close()

	out, err := runWithCapturedStdout(t, []string{"--mode=remote", "--endpoint", server.URL, "--output=json", "ls", "--states", "--cache-details"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	states, ok := payload["states"].([]any)
	if !ok || len(states) != 1 {
		t.Fatalf("expected one state row, got %s", out)
	}
	state, ok := states[0].(map[string]any)
	if !ok {
		t.Fatalf("expected object row, got %#v", states[0])
	}
	for _, key := range []string{"last_used_at", "use_count", "min_retention_until"} {
		if _, ok := state[key]; !ok {
			t.Fatalf("expected %q in state row, got %s", key, out)
		}
	}
}
