package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
)

func TestRunLsInstancePrefixAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("id_prefix") != "deadbeef" {
			json.NewEncoder(w).Encode([]client.InstanceEntry{})
			return
		}
		resp := []client.InstanceEntry{
			{InstanceID: "deadbeef111111111111111111111111"},
			{InstanceID: "deadbeef222222222222222222222222"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "deadbeef",
	}
	_, err := RunLs(context.Background(), opts)
	if err == nil {
		t.Fatalf("expected ambiguous prefix error")
	}
	var ambErr *AmbiguousPrefixError
	if !errors.As(err, &ambErr) {
		t.Fatalf("expected AmbiguousPrefixError, got %v", err)
	}
}

func TestRunLsInstancePrefixNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.InstanceEntry{})
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "beefbeef",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 0 {
		t.Fatalf("expected empty instances, got %+v", result.Instances)
	}
}

func TestRunLsStatePrefixUnique(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			http.NotFound(w, r)
			return
		}
		resp := []client.StateEntry{
			{StateID: "cafef00d11111111111111111111111111111111111111111111111111111111"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
		FilterState:   "cafef00d",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.States == nil || len(*result.States) != 1 {
		t.Fatalf("expected 1 state, got %+v", result.States)
	}
}

func TestRunLsStatePrefixAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			http.NotFound(w, r)
			return
		}
		resp := []client.StateEntry{
			{StateID: "cafef00d11111111111111111111111111111111111111111111111111111111"},
			{StateID: "cafef00d22222222222222222222222222222222222222222222222222222222"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
		FilterState:   "cafef00d",
	}
	_, err := RunLs(context.Background(), opts)
	var ambErr *AmbiguousPrefixError
	if !errors.As(err, &ambErr) {
		t.Fatalf("expected AmbiguousPrefixError, got %v", err)
	}
}

func TestPrintLsShortAndLongIDs(t *testing.T) {
	result := LsResult{
		Instances: &[]client.InstanceEntry{
			{
				InstanceID: "ABCDEF1234567890ABCDEF1234567890",
				ImageID:    "image-1",
				StateID:    "1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF",
				CreatedAt:  "2025-01-01T00:00:00Z",
				Status:     "active",
			},
		},
	}

	var shortBuf bytes.Buffer
	PrintLs(&shortBuf, result, LsPrintOptions{NoHeader: true})
	shortOut := shortBuf.String()
	if !strings.Contains(shortOut, "abcdef123456") {
		t.Fatalf("expected truncated lowercase id, got %q", shortOut)
	}

	var longBuf bytes.Buffer
	PrintLs(&longBuf, result, LsPrintOptions{NoHeader: true, LongIDs: true})
	longOut := longBuf.String()
	if !strings.Contains(longOut, "abcdef1234567890abcdef1234567890") {
		t.Fatalf("expected full lowercase id, got %q", longOut)
	}
}
