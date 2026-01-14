package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthAddsSchemeAndPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	cli := New(host, Options{Timeout: time.Second})
	_, err := cli.Health(context.Background())
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if gotPath != "/v1/health" {
		t.Fatalf("expected /v1/health path, got %q", gotPath)
	}
}

func TestHealthNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.Health(context.Background())
	if err == nil {
		t.Fatalf("expected health error")
	}
}
