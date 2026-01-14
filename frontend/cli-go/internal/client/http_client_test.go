package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestListNamesAddsFilters(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	_, err := cli.ListNames(context.Background(), ListFilters{
		Name:     "devdb",
		Instance: "i-1",
		State:    "s-1",
		Image:    "img-1",
	})
	if err != nil {
		t.Fatalf("list names failed: %v", err)
	}
	if gotPath != "/v1/names" {
		t.Fatalf("expected /v1/names path, got %q", gotPath)
	}
	if gotQuery.Get("name") != "devdb" || gotQuery.Get("instance") != "i-1" || gotQuery.Get("state") != "s-1" || gotQuery.Get("image") != "img-1" {
		t.Fatalf("unexpected query params: %v", gotQuery.Encode())
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
}
