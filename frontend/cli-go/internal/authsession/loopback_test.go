package authsession

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLoopbackServerProviderReceivesOneCallback(t *testing.T) {
	session, err := LoopbackServerProvider{}.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	resp, err := http.Get(session.RedirectURI() + "?state=s&code=c")
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "login complete") {
		t.Fatalf("callback response status=%d body=%q", resp.StatusCode, string(body))
	}

	values, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if values.Get("state") != "s" || values.Get("code") != "c" {
		t.Fatalf("callback values = %v", values)
	}
}

func TestLoopbackCloseSignalsContextWatcher(t *testing.T) {
	session, err := LoopbackServerProvider{}.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	concrete, ok := session.(*loopbackServerSession)
	if !ok {
		t.Fatalf("unexpected session type %T", session)
	}
	closed := concrete.closed

	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatalf("session close did not signal closed channel")
	}
}

func TestLoopbackServerRejectsNonLoopbackHostAndDuplicateCallback(t *testing.T) {
	session := &loopbackServerSession{
		server: &http.Server{},
		values: make(chan url.Values, 1),
		errs:   make(chan error, 1),
	}

	badHostReq := httptest.NewRequest(http.MethodGet, "http://example.com/?code=c", nil)
	badHostReq.Host = "localhost:12345"
	badHostResp := httptest.NewRecorder()
	session.ServeHTTP(badHostResp, badHostReq)
	if badHostResp.Code != http.StatusBadRequest {
		t.Fatalf("bad host status = %d, want 400", badHostResp.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:12345/?code=c", nil)
	resp := httptest.NewRecorder()
	session.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("first callback status = %d, want 200", resp.Code)
	}
	dup := httptest.NewRecorder()
	session.ServeHTTP(dup, req)
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate callback status = %d, want 409", dup.Code)
	}
}

func TestLoopbackWaitReturnsContextError(t *testing.T) {
	session := &loopbackServerSession{
		values: make(chan url.Values),
		errs:   make(chan error),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	if _, err := session.Wait(ctx); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestLoopbackWaitReturnsServerError(t *testing.T) {
	session := &loopbackServerSession{
		values: make(chan url.Values),
		errs:   make(chan error, 1),
	}
	session.errs <- errors.New("serve failed")
	if _, err := session.Wait(context.Background()); err == nil || !strings.Contains(err.Error(), "serve failed") {
		t.Fatalf("expected server error, got %v", err)
	}
}
