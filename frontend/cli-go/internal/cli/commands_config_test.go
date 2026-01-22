package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

func TestRunConfigGetRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	value, err := RunConfigGet(context.Background(), ConfigOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Path:     "features.flag",
	})
	if err != nil {
		t.Fatalf("RunConfigGet: %v", err)
	}
	cfg, ok := value.(client.ConfigValue)
	if !ok || cfg.Path != "features.flag" {
		t.Fatalf("unexpected config value: %#v", value)
	}
}

func TestRunConfigGetRemoteRequiresEndpoint(t *testing.T) {
	if _, err := RunConfigGet(context.Background(), ConfigOptions{Mode: "remote"}); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigGetLocalAuto(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	prev := connectOrStart
	connectOrStart = func(ctx context.Context, opts daemon.ConnectOptions) (daemon.ConnectResult, error) {
		return daemon.ConnectResult{Endpoint: server.URL, AuthToken: "token-1"}, nil
	}
	t.Cleanup(func() { connectOrStart = prev })

	if _, err := RunConfigGet(context.Background(), ConfigOptions{
		Mode:     "local",
		Endpoint: "auto",
		Timeout:  time.Second,
		Verbose:  true,
		Path:     "features.flag",
	}); err != nil {
		t.Fatalf("RunConfigGet: %v", err)
	}
}

func TestRunConfigSetRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	value, err := RunConfigSet(context.Background(), ConfigOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Path:     "features.flag",
		Value:    true,
	})
	if err != nil {
		t.Fatalf("RunConfigSet: %v", err)
	}
	if value.Path != "features.flag" {
		t.Fatalf("unexpected config value: %#v", value)
	}
}

func TestRunConfigSetRemoteMissingEndpoint(t *testing.T) {
	if _, err := RunConfigSet(context.Background(), ConfigOptions{Mode: "remote"}); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigRemoveRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" || r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":null}`)
	}))
	defer server.Close()

	value, err := RunConfigRemove(context.Background(), ConfigOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Path:     "features.flag",
	})
	if err != nil {
		t.Fatalf("RunConfigRemove: %v", err)
	}
	if value.Path != "features.flag" {
		t.Fatalf("unexpected config value: %#v", value)
	}
}

func TestRunConfigRemoveRemoteMissingEndpoint(t *testing.T) {
	if _, err := RunConfigRemove(context.Background(), ConfigOptions{Mode: "remote"}); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestRunConfigSchemaLocalEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config/schema" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"type":"object"}`)
	}))
	defer server.Close()

	value, err := RunConfigSchema(context.Background(), ConfigOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunConfigSchema: %v", err)
	}
	schema, ok := value.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("unexpected schema: %#v", value)
	}
}

func TestRunConfigSchemaRemoteMissingEndpoint(t *testing.T) {
	if _, err := RunConfigSchema(context.Background(), ConfigOptions{Mode: "remote"}); err == nil {
		t.Fatalf("expected error for missing endpoint")
	}
}

func TestConfigClientRemoteAutoError(t *testing.T) {
	if _, err := configClient(context.Background(), ConfigOptions{Mode: "remote", Endpoint: "auto"}); err == nil {
		t.Fatalf("expected error for auto endpoint in remote mode")
	}
}

func TestConfigClientCustomModeUsesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/config" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"features.flag","value":true}`)
	}))
	defer server.Close()

	client, err := configClient(context.Background(), ConfigOptions{Mode: "custom", Endpoint: server.URL})
	if err != nil {
		t.Fatalf("configClient: %v", err)
	}
	if _, err := client.GetConfig(context.Background(), "features.flag", false); err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
}
