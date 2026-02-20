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

func TestClientCoverageHealthAndRunCommandSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"exit","exit_code":0}`+"\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	health, err := cli.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !health.Ok {
		t.Fatalf("expected ok health, got %+v", health)
	}

	stream, err := cli.RunCommand(context.Background(), RunRequest{InstanceRef: "inst", Kind: "psql"})
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !strings.Contains(string(data), `"type":"exit"`) {
		t.Fatalf("unexpected stream: %q", string(data))
	}
}

func TestClientCoverageListAndConfigErrorPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/names", "/v1/instances", "/v1/prepare-jobs", "/v1/tasks", "/v1/config", "/v1/config/schema":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"message":"boom"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})

	if _, err := cli.ListNames(context.Background(), ListFilters{}); err == nil {
		t.Fatalf("expected ListNames error")
	}
	if _, err := cli.ListInstances(context.Background(), ListFilters{}); err == nil {
		t.Fatalf("expected ListInstances error")
	}
	if _, err := cli.ListPrepareJobs(context.Background(), "job-1"); err == nil {
		t.Fatalf("expected ListPrepareJobs error")
	}
	if _, err := cli.ListTasks(context.Background(), "job-1"); err == nil {
		t.Fatalf("expected ListTasks error")
	}
	if _, err := cli.GetConfig(context.Background(), "", false); err == nil {
		t.Fatalf("expected GetConfig(map) error")
	}
	if _, err := cli.GetConfig(context.Background(), "dbms.image", false); err == nil {
		t.Fatalf("expected GetConfig(path) error")
	}
	if _, err := cli.GetConfigSchema(context.Background()); err == nil {
		t.Fatalf("expected GetConfigSchema error")
	}
}

func TestClientCoverageSetAndRemoveConfigBranches(t *testing.T) {
	t.Run("SetConfig marshal error", func(t *testing.T) {
		cli := New("http://127.0.0.1:1", Options{Timeout: time.Second})
		_, err := cli.SetConfig(context.Background(), ConfigValue{
			Path:  "dbms.image",
			Value: map[string]any{"ch": make(chan int)},
		})
		if err == nil {
			t.Fatalf("expected marshal error")
		}
	})

	t.Run("SetConfig non-200 and decode error", func(t *testing.T) {
		serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/config" || r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"message":"bad set"}`)
		}))
		t.Cleanup(serverErr.Close)

		cli := New(serverErr.URL, Options{Timeout: time.Second})
		if _, err := cli.SetConfig(context.Background(), ConfigValue{Path: "dbms.image", Value: "pg"}); err == nil {
			t.Fatalf("expected non-200 error")
		}

		serverDecode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/config" || r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "{")
		}))
		t.Cleanup(serverDecode.Close)

		cliDecode := New(serverDecode.URL, Options{Timeout: time.Second})
		if _, err := cliDecode.SetConfig(context.Background(), ConfigValue{Path: "dbms.image", Value: "pg"}); err == nil {
			t.Fatalf("expected decode error")
		}
	})

	t.Run("RemoveConfig non-200 and decode error", func(t *testing.T) {
		serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/config" || r.Method != http.MethodDelete {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"message":"bad remove"}`)
		}))
		t.Cleanup(serverErr.Close)

		cli := New(serverErr.URL, Options{Timeout: time.Second})
		if _, err := cli.RemoveConfig(context.Background(), "dbms.image"); err == nil {
			t.Fatalf("expected non-200 error")
		}

		serverDecode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/config" || r.Method != http.MethodDelete {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "{")
		}))
		t.Cleanup(serverDecode.Close)

		cliDecode := New(serverDecode.URL, Options{Timeout: time.Second})
		if _, err := cliDecode.RemoveConfig(context.Background(), "dbms.image"); err == nil {
			t.Fatalf("expected decode error")
		}
	})
}

func TestClientCoverageInvalidBaseURLRequestErrors(t *testing.T) {
	bad := &Client{
		baseURL: "://bad",
		http:    &http.Client{Timeout: time.Second},
	}

	if _, _, err := bad.GetInstance(context.Background(), "inst"); err == nil {
		t.Fatalf("expected GetInstance request error")
	}
	if _, err := bad.RunCommand(context.Background(), RunRequest{InstanceRef: "inst", Kind: "psql"}); err == nil {
		t.Fatalf("expected RunCommand request error")
	}
	if _, err := bad.SetConfig(context.Background(), ConfigValue{Path: "dbms.image", Value: "pg"}); err == nil {
		t.Fatalf("expected SetConfig request error")
	}
	if _, err := bad.RemoveConfig(context.Background(), "dbms.image"); err == nil {
		t.Fatalf("expected RemoveConfig request error")
	}
	if _, _, err := bad.DeleteState(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected DeleteState request error")
	}
	if _, err := bad.GetConfigSchema(context.Background()); err == nil {
		t.Fatalf("expected GetConfigSchema request error")
	}
}

func TestClientCoverageStreamPrepareEventsInvalidURL(t *testing.T) {
	cli := New("http://127.0.0.1:1234", Options{Timeout: time.Second})
	if _, err := cli.StreamPrepareEvents(context.Background(), "\x00", ""); err == nil {
		t.Fatalf("expected invalid URL error")
	}
}
