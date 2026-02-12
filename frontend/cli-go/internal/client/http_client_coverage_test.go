package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCheckRedirectCopiesHeadersAndLimits(t *testing.T) {
	cli := New("http://example.com", Options{})

	next, _ := http.NewRequest(http.MethodGet, "http://example.com/next", nil)
	if err := cli.http.CheckRedirect(next, nil); err != nil {
		t.Fatalf("unexpected error for empty via: %v", err)
	}

	prev, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	prev.Header.Set("Authorization", "Bearer token")
	prev.Header.Set("User-Agent", "agent")
	next, _ = http.NewRequest(http.MethodGet, "http://example.com/next", nil)
	if err := cli.http.CheckRedirect(next, []*http.Request{prev}); err != nil {
		t.Fatalf("unexpected redirect error: %v", err)
	}
	if got := next.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("expected auth header propagated, got %q", got)
	}
	if got := next.Header.Get("User-Agent"); got != "agent" {
		t.Fatalf("expected user agent propagated, got %q", got)
	}

	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = prev
	}
	if err := cli.http.CheckRedirect(next, via); err == nil {
		t.Fatalf("expected redirect limit error")
	}
}

func TestClientDoJSONErrorStatusAndDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			if r.URL.Query().Get("bad") == "1" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "not-json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	if _, err := cli.Health(context.Background()); err == nil {
		t.Fatalf("expected decode error")
	}
	if _, err := cli.doRequest(context.Background(), http.MethodGet, "/v1/health?bad=1", false); err != nil {
		t.Fatalf("unexpected request error: %v", err)
	}
	err := cli.doJSON(context.Background(), http.MethodGet, "/v1/health?bad=1", false, &HealthResponse{})
	if err == nil {
		t.Fatalf("expected status error")
	}
	if _, ok := err.(*HTTPStatusError); !ok {
		t.Fatalf("expected HTTPStatusError, got %T", err)
	}
}

func TestClientDoJSONOptionalStatusAndDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/names/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/names/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/v1/names/bad":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "not-json")
		case "/v1/names/name-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"name":"name-1","status":"ready"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	_, found, err := cli.GetName(context.Background(), "missing")
	if err != nil || found {
		t.Fatalf("expected not found without error, found=%v err=%v", found, err)
	}
	_, _, err = cli.GetName(context.Background(), "error")
	if err == nil {
		t.Fatalf("expected error for non-404")
	}
	_, _, err = cli.GetName(context.Background(), "bad")
	if err == nil {
		t.Fatalf("expected decode error")
	}
	_, found, err = cli.GetName(context.Background(), "name-1")
	if err != nil || !found {
		t.Fatalf("expected found name, found=%v err=%v", found, err)
	}
}

func TestClientDeleteWithOptionsErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/instances/boom":
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"message":"bad"}`)
		case "/v1/states/badjson":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "not-json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	if _, _, err := cli.DeleteInstance(context.Background(), "boom", DeleteOptions{}); err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("expected error response, got %v", err)
	}
	if _, _, err := cli.DeleteState(context.Background(), "badjson", DeleteOptions{}); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestClientDoRequestInvalidURL(t *testing.T) {
	cli := New("http://[::1", Options{})
	if _, err := cli.doRequest(context.Background(), http.MethodGet, "/v1/health", false); err == nil {
		t.Fatalf("expected invalid url error")
	}
}

func TestClientDoRequestWithBodyHeaders(t *testing.T) {
	var gotAuth string
	var gotUA string
	var gotType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "agent"})
	resp, err := cli.doRequestWithBody(context.Background(), http.MethodPost, "/v1/config", true, bytes.NewBufferString("{}"), "application/json")
	if err != nil {
		t.Fatalf("doRequestWithBody: %v", err)
	}
	resp.Body.Close()
	if gotAuth != "Bearer token" || gotUA != "agent" || gotType != "application/json" {
		t.Fatalf("unexpected headers auth=%q ua=%q type=%q", gotAuth, gotUA, gotType)
	}
}

func TestClientRunCommandErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/runs" {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"message":"boom"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.RunCommand(context.Background(), RunRequest{InstanceRef: "inst"})
	if err == nil {
		t.Fatalf("expected error for non-OK status")
	}
}

func TestClientGetConfigSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/config/schema" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"type": "object"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	value, err := cli.GetConfigSchema(context.Background())
	if err != nil {
		t.Fatalf("GetConfigSchema: %v", err)
	}
	if _, ok := value.(map[string]any); !ok {
		t.Fatalf("expected map schema, got %T", value)
	}
}

func TestParseErrorResponseEmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader("")),
	}
	err := parseErrorResponse(resp)
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(*HTTPStatusError); !ok {
		t.Fatalf("expected HTTPStatusError, got %T", err)
	}
}
