package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sqlrs/engine/internal/config"
)

func TestConfigGetValue(t *testing.T) {
	cfg := &fakeConfig{getValue: "value"}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?path=features.flag&effective=true", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if cfg.lastPath != "features.flag" || !cfg.lastEffective {
		t.Fatalf("unexpected config call: %q %v", cfg.lastPath, cfg.lastEffective)
	}
	var payload config.Value
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Path != "features.flag" || payload.Value != "value" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestConfigGetRoot(t *testing.T) {
	cfg := &fakeConfig{getValue: map[string]any{"a": "b"}}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?effective=true", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if cfg.lastPath != "" || !cfg.lastEffective {
		t.Fatalf("unexpected config call: %q %v", cfg.lastPath, cfg.lastEffective)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["a"] != "b" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestConfigGetMissingPath(t *testing.T) {
	cfg := &fakeConfig{getErr: config.ErrPathNotFound}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config?path=missing", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestConfigSetValue(t *testing.T) {
	cfg := &fakeConfig{setValue: true}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	body := bytes.NewBufferString(`{"path":"features.flag","value":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/v1/config", body)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if cfg.lastPath != "features.flag" {
		t.Fatalf("unexpected path: %q", cfg.lastPath)
	}
	if value, ok := cfg.lastValue.(bool); !ok || value != true {
		t.Fatalf("unexpected value: %#v", cfg.lastValue)
	}
	var payload config.Value
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Path != "features.flag" || payload.Value != true {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestConfigRemoveValue(t *testing.T) {
	cfg := &fakeConfig{removeValue: nil}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/config?path=features.flag", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if cfg.lastPath != "features.flag" {
		t.Fatalf("unexpected path: %q", cfg.lastPath)
	}
	var payload config.Value
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Path != "features.flag" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestConfigSchema(t *testing.T) {
	cfg := &fakeConfig{schema: map[string]any{"type": "object"}}
	handler := NewHandler(Options{
		AuthToken: "secret",
		Config:    cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config/schema", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["type"] != "object" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

type fakeConfig struct {
	lastPath      string
	lastEffective bool
	lastValue     any
	getValue      any
	getErr        error
	setValue      any
	setErr        error
	removeValue   any
	removeErr     error
	schema        any
}

func (f *fakeConfig) Get(path string, effective bool) (any, error) {
	f.lastPath = path
	f.lastEffective = effective
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeConfig) Set(path string, value any) (any, error) {
	f.lastPath = path
	f.lastValue = value
	if f.setErr != nil {
		return nil, f.setErr
	}
	if f.setValue != nil || value == nil {
		return f.setValue, nil
	}
	return value, nil
}

func (f *fakeConfig) Remove(path string) (any, error) {
	f.lastPath = path
	if f.removeErr != nil {
		return nil, f.removeErr
	}
	return f.removeValue, nil
}

func (f *fakeConfig) Schema() any {
	return f.schema
}
