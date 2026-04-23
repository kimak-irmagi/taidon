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

func TestCacheExplainRequiresAuth(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	handler := NewHandler(opts)
	req := httptest.NewRequest(http.MethodPost, "/v1/cache/explain/prepare", bytes.NewBufferString(`{}`))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func TestCacheExplainRejectsInvalidJSON(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	handler := NewHandler(opts)
	req := httptest.NewRequest(http.MethodPost, "/v1/cache/explain/prepare", bytes.NewBufferString(`{`))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestCacheExplainReturnsPayload(t *testing.T) {
	opts, cleanup := newRouteTestOptions(t)
	defer cleanup()

	scriptPath := filepath.Join(t.TempDir(), "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	body := bytes.NewBufferString(`{"prepare_kind":"psql","image_id":"image-1","psql_args":["-f","` + filepath.ToSlash(scriptPath) + `"]}`)
	handler := NewHandler(opts)
	req := httptest.NewRequest(http.MethodPost, "/v1/cache/explain/prepare", body)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["decision"] != "miss" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if _, ok := payload["signature"]; !ok {
		t.Fatalf("expected signature in payload, got %+v", payload)
	}
}
