package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare"
)

func TestWriteJSONSetsContentType(t *testing.T) {
	resp := httptest.NewRecorder()

	if err := writeJSON(resp, map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	if got := resp.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestWriteJSONStatusReturnsWriteError(t *testing.T) {
	writer := &failingResponseWriter{
		header: make(http.Header),
		err:    errors.New("boom"),
	}

	err := writeJSONStatus(writer, map[string]string{"status": "ok"}, http.StatusCreated)
	if !errors.Is(err, writer.err) {
		t.Fatalf("expected write error, got %v", err)
	}
	if writer.status != http.StatusCreated {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusCreated)
	}
}

func TestWriteErrorResponseWritesPrepareErrorPayload(t *testing.T) {
	resp := httptest.NewRecorder()

	if err := writeErrorResponse(resp, "invalid_argument", "bad request", "details", http.StatusBadRequest); err != nil {
		t.Fatalf("writeErrorResponse: %v", err)
	}

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	var payload prepare.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "invalid_argument" || payload.Message != "bad request" || payload.Details != "details" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestStreamPrepareEventsWithoutFlusherReturnsInternalServerError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/prepare-jobs/job/events", nil)
	writer := &failingResponseWriter{header: make(http.Header)}

	streamPrepareEvents(writer, req, nil, "job")

	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusInternalServerError)
	}
}

type failingResponseWriter struct {
	header http.Header
	status int
	err    error
}

func (w *failingResponseWriter) Header() http.Header {
	return w.header
}

func (w *failingResponseWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return len(data), nil
}

func (w *failingResponseWriter) WriteHeader(status int) {
	w.status = status
}
