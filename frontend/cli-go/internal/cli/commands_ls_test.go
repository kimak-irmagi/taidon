package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
)

func TestPrintLsQuietSeparatesSections(t *testing.T) {
	result := LsResult{
		Names: &[]client.NameEntry{
			{
				Name:    "dev",
				ImageID: "image-1",
				StateID: "state-1",
				Status:  "active",
			},
		},
		Instances: &[]client.InstanceEntry{
			{
				InstanceID: "instance-1",
				ImageID:    "image-1",
				StateID:    "state-1",
				CreatedAt:  "2025-01-01T00:00:00Z",
				Status:     "active",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true})

	out := buf.String()
	if strings.Contains(out, "Names") || strings.Contains(out, "Instances") {
		t.Fatalf("unexpected section titles in quiet output: %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatalf("expected blank line between sections, got %q", out)
	}
}

func TestRunLsUsesNameItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"dev","image_id":"img","state_id":"state","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/names/dev" {
		t.Fatalf("expected name item endpoint, got %q", gotPath)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names result: %+v", result.Names)
	}
}

func TestRunLsUsesInstanceItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances/dev" {
		t.Fatalf("expected instance item endpoint, got %q", gotPath)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}

func TestRunLsUsesInstancePrefixListEndpoint(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "abc12345",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances" {
		t.Fatalf("expected list instances endpoint, got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "id_prefix=abc12345") {
		t.Fatalf("expected id_prefix in query, got %q", gotQuery)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}
