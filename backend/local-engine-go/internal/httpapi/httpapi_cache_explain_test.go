package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheExplainHTTPAPIResponseShape(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	scriptPath := filepath.Join(t.TempDir(), "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	handler := NewHandler(opts)
	body := bytes.NewBufferString(`{"prepare_kind":"psql","image_id":"image-1","psql_args":["-f","` + filepath.ToSlash(scriptPath) + `"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/cache/explain/prepare", body)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want %q", got, "application/json")
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["decision"] != "miss" {
		t.Fatalf("unexpected decision payload: %+v", payload)
	}
	if payload["reason_code"] != "no_matching_state" {
		t.Fatalf("unexpected reason payload: %+v", payload)
	}
	if _, ok := payload["signature"]; !ok {
		t.Fatalf("expected signature in payload, got %+v", payload)
	}
	if payload["resolved_image_id"] != "image-1@sha256:resolved" {
		t.Fatalf("unexpected resolved image payload: %+v", payload)
	}
}
