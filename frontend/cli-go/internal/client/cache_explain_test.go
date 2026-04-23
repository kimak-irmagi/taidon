package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExplainPrepareCache(t *testing.T) {
	var gotAuth string
	var gotReq PrepareJobRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/cache/explain/prepare" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"hit","reason_code":"exact_state_match","signature":"sig-1","matched_state_id":"state-1","resolved_image_id":"image@sha256:resolved"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "secret"})
	result, err := cli.ExplainPrepareCache(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", "prepare.sql"},
	})
	if err != nil {
		t.Fatalf("ExplainPrepareCache: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
	if gotReq.PrepareKind != "psql" || gotReq.ImageID != "image-1" {
		t.Fatalf("unexpected request: %+v", gotReq)
	}
	if result.Decision != "hit" || result.ReasonCode != "exact_state_match" {
		t.Fatalf("unexpected explain result: %+v", result)
	}
	if result.Signature != "sig-1" || result.MatchedStateID != "state-1" {
		t.Fatalf("unexpected explain cache payload: %+v", result)
	}
	if result.ResolvedImageID != "image@sha256:resolved" {
		t.Fatalf("unexpected resolved image id: %+v", result)
	}
}

func TestExplainPrepareCacheLiquibase(t *testing.T) {
	var gotReq PrepareJobRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/cache/explain/prepare" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":"miss","reason_code":"no_matching_state","signature":"sig-lb"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	result, err := cli.ExplainPrepareCache(context.Background(), PrepareJobRequest{
		PrepareKind:       "lb",
		ImageID:           "image-1",
		LiquibaseArgs:     []string{"update", "--changelog-file", "db/changelog.xml"},
		LiquibaseExec:     "liquibase",
		LiquibaseExecMode: "native",
		WorkDir:           "/workspace",
	})
	if err != nil {
		t.Fatalf("ExplainPrepareCache: %v", err)
	}
	if gotReq.PrepareKind != "lb" {
		t.Fatalf("expected lb request, got %+v", gotReq)
	}
	if len(gotReq.LiquibaseArgs) != 3 || gotReq.WorkDir != "/workspace" {
		t.Fatalf("unexpected liquibase request: %+v", gotReq)
	}
	if result.Decision != "miss" || result.ReasonCode != "no_matching_state" || result.Signature != "sig-lb" {
		t.Fatalf("unexpected explain result: %+v", result)
	}
}

func TestExplainPrepareCacheError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid prepare request"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, err := cli.ExplainPrepareCache(context.Background(), PrepareJobRequest{
		PrepareKind: "psql",
		ImageID:     "image-1",
	})
	if err == nil || err.Error() != "invalid prepare request" {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}
