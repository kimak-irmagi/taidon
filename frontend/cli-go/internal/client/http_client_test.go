package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveURL(t *testing.T) {
	cli := New("http://127.0.0.1:1234/", Options{})
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "  ", want: ""},
		{raw: "/v1/events", want: "http://127.0.0.1:1234/v1/events"},
		{raw: "v1/events", want: "http://127.0.0.1:1234/v1/events"},
		{raw: "http://example.com/x", want: "http://example.com/x"},
		{raw: "https://example.com", want: "https://example.com"},
	}
	for _, tc := range cases {
		if got := cli.resolveURL(tc.raw); got != tc.want {
			t.Fatalf("resolveURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestStreamPrepareEventsRejectsEmptyURL(t *testing.T) {
	cli := New("http://127.0.0.1:1234", Options{})
	if _, err := cli.StreamPrepareEvents(context.Background(), " ", ""); err == nil {
		t.Fatalf("expected error for empty url")
	}
}

func TestStreamPrepareEventsRelativeURL(t *testing.T) {
	var gotRange string
	var gotAuth string
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotRange = r.Header.Get("Range")
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "sqlrs-cli"})
	resp, err := cli.StreamPrepareEvents(context.Background(), "v1/prepare/events", "items=1-2")
	if err != nil {
		t.Fatalf("StreamPrepareEvents: %v", err)
	}
	resp.Body.Close()
	if gotRange != "items=1-2" {
		t.Fatalf("expected range header, got %q", gotRange)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if gotUA != "sqlrs-cli" {
		t.Fatalf("expected user-agent, got %q", gotUA)
	}
}

func TestStreamPrepareEventsAbsoluteURL(t *testing.T) {
	var sawRange bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/prepare/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Range") != "" {
			sawRange = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "sqlrs-cli"})
	resp, err := cli.StreamPrepareEvents(context.Background(), server.URL+"/v1/prepare/events", "")
	if err != nil {
		t.Fatalf("StreamPrepareEvents: %v", err)
	}
	resp.Body.Close()
	if sawRange {
		t.Fatalf("expected no range header")
	}
}
