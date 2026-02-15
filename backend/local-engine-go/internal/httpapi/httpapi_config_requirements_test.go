package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"sqlrs/engine/internal/config"
)

func TestConfigRoutesRequireConfigStore(t *testing.T) {
	handler := NewHandler(Options{AuthToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?path=feature.flag", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for /v1/config without store, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/config/schema", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for /v1/config/schema without store, got %d", resp.Code)
	}
}

func TestConfigRoutesRejectUnsupportedMethods(t *testing.T) {
	cfg := &fakeConfig{}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodPut, "/v1/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for /v1/config PUT, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/config/schema", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for /v1/config/schema POST, got %d", resp.Code)
	}
}

func TestConfigGetRejectsInvalidEffectiveQuery(t *testing.T) {
	cfg := &fakeConfig{}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?path=feature.flag&effective=maybe", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid effective query, got %d", resp.Code)
	}
}

func TestConfigGetRejectsInvalidPath(t *testing.T) {
	cfg := &fakeConfig{getErr: config.ErrInvalidPath}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?path=invalid", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid config path, got %d", resp.Code)
	}
}

func TestConfigSetRejectsInvalidPayloads(t *testing.T) {
	cfg := &fakeConfig{}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/config", bytes.NewBufferString("{bad-json"))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", resp.Code)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/config", bytes.NewBufferString(`{"path":"  ","value":1}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty path, got %d", resp.Code)
	}
}

func TestConfigSetRejectsInvalidValueAndInternalError(t *testing.T) {
	cfg := &fakeConfig{setErr: config.ErrInvalidValue}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodPatch, "/v1/config", bytes.NewBufferString(`{"path":"feature.flag","value":"bad"}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid config value, got %d", resp.Code)
	}

	cfg.setErr = errors.New("write failed")
	req = httptest.NewRequest(http.MethodPatch, "/v1/config", bytes.NewBufferString(`{"path":"feature.flag","value":"ok"}`))
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for storage error, got %d", resp.Code)
	}
}

func TestConfigDeleteRejectsInvalidRequests(t *testing.T) {
	cfg := &fakeConfig{}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty path, got %d", resp.Code)
	}

	cfg.removeErr = config.ErrInvalidPath
	req = httptest.NewRequest(http.MethodDelete, "/v1/config?path=bad", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid path, got %d", resp.Code)
	}

	cfg.removeErr = errors.New("delete failed")
	req = httptest.NewRequest(http.MethodDelete, "/v1/config?path=feature.flag", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for storage error, got %d", resp.Code)
	}
}
