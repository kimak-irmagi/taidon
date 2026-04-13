package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRunStatusJSONIncludesCacheSummary(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"version":"v1","instanceId":"inst","pid":1}`)
		case "/v1/cache/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"usage_bytes":2048,"configured_max_bytes":4096,"effective_max_bytes":3072,"reserve_bytes":512,"high_watermark":0.9,"low_watermark":0.8,"min_state_age":"10m","store_total_bytes":8192,"store_free_bytes":6144,"state_count":3,"reclaimable_bytes":1024,"blocked_count":1,"pressure_reasons":["usage_above_high_watermark"]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	out, err := runWithCapturedStdout(t, []string{"--mode=remote", "--endpoint", server.URL, "--output=json", "status"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if _, ok := payload["cacheSummary"]; !ok {
		t.Fatalf("expected cacheSummary in output, got %s", out)
	}
	if _, ok := payload["cacheDetails"]; ok {
		t.Fatalf("did not expect cacheDetails without --cache, got %s", out)
	}
}

func TestRunStatusJSONWithCacheFlagIncludesCacheDetails(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"version":"v1","instanceId":"inst","pid":1}`)
		case "/v1/cache/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"usage_bytes":2048,"configured_max_bytes":4096,"effective_max_bytes":3072,"reserve_bytes":512,"high_watermark":0.9,"low_watermark":0.8,"min_state_age":"10m","store_total_bytes":8192,"store_free_bytes":6144,"state_count":3,"reclaimable_bytes":1024,"blocked_count":1,"pressure_reasons":["usage_above_high_watermark"],"last_eviction":{"completed_at":"2026-03-09T12:00:00Z","trigger":"post_snapshot","evicted_count":2,"freed_bytes":256,"blocked_count":1,"reclaimable_bytes":1024,"usage_bytes_before":2304,"usage_bytes_after":2048,"free_bytes_before":5888,"free_bytes_after":6144}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	out, err := runWithCapturedStdout(t, []string{"--mode=remote", "--endpoint", server.URL, "--output=json", "status", "--cache"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if _, ok := payload["cacheSummary"]; !ok {
		t.Fatalf("expected cacheSummary in output, got %s", out)
	}
	if _, ok := payload["cacheDetails"]; !ok {
		t.Fatalf("expected cacheDetails in output, got %s", out)
	}
}

func TestRunStatusCacheFlagFailsWhenDiagnosticsUnavailable(t *testing.T) {
	temp := t.TempDir()
	setTestDirs(t, temp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"version":"v1"}`)
		case "/v1/cache/status":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := Run([]string{"--mode=remote", "--endpoint", server.URL, "status", "--cache"}); err == nil {
		t.Fatalf("expected --cache failure")
	}
}

func runWithCapturedStdout(t *testing.T, args []string) (string, error) {
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
	defer func() {
		_ = r.Close()
	}()

	runErr := Run(args)
	_ = w.Close()

	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	captured := string(data)
	t.Cleanup(func() {
		if !t.Failed() || captured == "" {
			return
		}
		fmt.Fprintf(oldStdout, "\n[%s] captured stdout:\n%s\n", t.Name(), captured)
	})
	return captured, runErr
}
