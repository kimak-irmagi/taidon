package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreatePrepareJobParsesSourceInputsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"code":"source_inputs_missing","message":"missing","missing_manifest_entries":[{"path":"query.sql","kind":"file_hash"}],"missing_blobs":[{"path":"query.sql","hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`)
	}))
	t.Cleanup(server.Close)

	_, err := New(server.URL, Options{}).CreatePrepareJob(context.Background(), PrepareJobRequest{PrepareKind: "psql"})
	var missing *SourceInputsMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected SourceInputsMissingError, got %T %v", err, err)
	}
	if missing.StatusCode != http.StatusConflict || len(missing.Response.MissingManifestEntries) != 1 || len(missing.Response.MissingBlobs) != 1 {
		t.Fatalf("unexpected missing response: %+v", missing)
	}
}

func TestPutSourceBlobSendsAuthenticatedOctetStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/v1/source-blobs/sha256/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("Content-Type = %q", got)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(data) != "source" {
			t.Fatalf("body = %q", string(data))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	err := New(server.URL, Options{AuthToken: "token"}).PutSourceBlob(
		context.Background(),
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		strings.NewReader("source"),
	)
	if err != nil {
		t.Fatalf("PutSourceBlob: %v", err)
	}
}

func TestPutSourceBlobParsesErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, `{"code":"source_blob_too_large","message":"too large"}`)
	}))
	t.Cleanup(server.Close)

	err := New(server.URL, Options{}).PutSourceBlob(
		context.Background(),
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		strings.NewReader("source"),
	)
	var responseErr *ErrorResponseError
	if !errors.As(err, &responseErr) || responseErr.Code != "source_blob_too_large" {
		t.Fatalf("expected source_blob_too_large ErrorResponseError, got %T %v", err, err)
	}
}
