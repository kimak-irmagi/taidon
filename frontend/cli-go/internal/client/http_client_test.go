package client

import (
	"context"
	"encoding/json"
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
	if gotQuery.Get("instance") != "i-1" || gotQuery.Get("state") != "s-1" || gotQuery.Get("image") != "img-1" {
		t.Fatalf("unexpected query params: %v", gotQuery.Encode())
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
}

func TestGetInstanceFollowsRedirectWithAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/instances/dev":
			w.Header().Set("Location", "/v1/instances/abc")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/v1/instances/abc":
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	entry, found, err := cli.GetInstance(context.Background(), "dev")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !found || entry.InstanceID != "abc" {
		t.Fatalf("unexpected instance entry: %+v", entry)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization on redirected request, got %q", gotAuth)
	}
}

func TestCreatePrepareJob(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotRequest PrepareJobRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1"}`)
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token"})
	accepted, err := cli.CreatePrepareJob(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", "/abs/init.sql"},
	})
	if err != nil {
		t.Fatalf("create prepare job: %v", err)
	}
	if gotPath != "/v1/prepare-jobs" {
		t.Fatalf("expected /v1/prepare-jobs path, got %q", gotPath)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
	if gotRequest.ImageID != "image-1" || gotRequest.PrepareKind != "psql" {
		t.Fatalf("unexpected request: %+v", gotRequest)
	}
	if accepted.JobID != "job-1" {
		t.Fatalf("unexpected accepted response: %+v", accepted)
	}
}
