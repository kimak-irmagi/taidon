package stream

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sample struct {
	ID string `json:"id"`
}

func TestWantsNDJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "Application/X-NDJSON")
	if !WantsNDJSON(req) {
		t.Fatalf("expected WantsNDJSON to be true")
	}
}

func TestWriteListNDJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/x-ndjson")
	rec := httptest.NewRecorder()

	items := []sample{{ID: "a"}, {ID: "b"}}
	if err := WriteList(rec, req, items); err != nil {
		t.Fatalf("WriteList: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/x-ndjson") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines, got %d", len(lines))
	}
}

func TestWriteListJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	items := []sample{{ID: "a"}}
	if err := WriteList(rec, req, items); err != nil {
		t.Fatalf("WriteList: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "\"id\":\"a\"") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestWriteListNDJSONError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/x-ndjson")
	writer := &errorWriter{}

	items := []sample{{ID: "a"}}
	if err := WriteList(writer, req, items); err == nil {
		t.Fatalf("expected error")
	}
}

type errorWriter struct {
	header http.Header
}

func (w *errorWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

func (w *errorWriter) WriteHeader(status int) {}
