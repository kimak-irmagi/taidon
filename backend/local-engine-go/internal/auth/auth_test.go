package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireBearerAllowsEmptyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	if !RequireBearer(rec, req, "") {
		t.Fatalf("expected allow with empty token")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected no status written, got %d", rec.Code)
	}
}

func TestRequireBearerAcceptsValidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	if !RequireBearer(rec, req, "secret") {
		t.Fatalf("expected allow with valid token")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected no status written, got %d", rec.Code)
	}
}

func TestRequireBearerRejectsInvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()

	if RequireBearer(rec, req, "secret") {
		t.Fatalf("expected reject with invalid token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
