package authsession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoogleOAuthClientExchangeRefreshAndRevoke(t *testing.T) {
	var tokenForms []string
	var revokedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost {
				t.Fatalf("token method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			tokenForms = append(tokenForms, r.Form.Encode())
			resp := map[string]any{
				"id_token":      "id-token",
				"refresh_token": "refresh-token",
				"access_token":  "access-token",
				"expires_in":    3600,
				"token_type":    "Bearer",
				"scope":         "openid email profile",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/revoke":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			revokedToken = r.Form.Get("token")
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := GoogleOAuthClient{
		TokenEndpoint:      server.URL + "/token",
		RevocationEndpoint: server.URL + "/revoke",
	}
	exchanged, err := client.ExchangeCode(context.Background(), CodeExchangeRequest{
		ClientID:     "client-id",
		Code:         "code-1",
		CodeVerifier: "verifier-1",
		RedirectURI:  "http://127.0.0.1:12345",
	})
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if exchanged.IDToken != "id-token" || exchanged.RefreshToken != "refresh-token" || exchanged.ExpiresIn != 3600 {
		t.Fatalf("exchange response = %+v", exchanged)
	}
	refreshed, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "refresh-old"})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken != "access-token" {
		t.Fatalf("refresh response = %+v", refreshed)
	}
	if err := client.Revoke(context.Background(), "refresh-old"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if len(tokenForms) != 2 {
		t.Fatalf("token requests = %d, want 2", len(tokenForms))
	}
	if !strings.Contains(tokenForms[0], "grant_type=authorization_code") || !strings.Contains(tokenForms[0], "code=code-1") {
		t.Fatalf("exchange form = %q", tokenForms[0])
	}
	if !strings.Contains(tokenForms[1], "grant_type=refresh_token") || !strings.Contains(tokenForms[1], "refresh_token=refresh-old") {
		t.Fatalf("refresh form = %q", tokenForms[1])
	}
	if revokedToken != "refresh-old" {
		t.Fatalf("revoked token = %q, want refresh-old", revokedToken)
	}
}

func TestGoogleOAuthClientTokenEndpointErrors(t *testing.T) {
	t.Run("http status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "invalid_grant", http.StatusBadRequest)
		}))
		defer server.Close()

		client := GoogleOAuthClient{TokenEndpoint: server.URL}
		_, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "bad"})
		if err == nil || !strings.Contains(err.Error(), "status 400") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "{")
		}))
		defer server.Close()

		client := GoogleOAuthClient{TokenEndpoint: server.URL}
		_, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "refresh"})
		if err == nil {
			t.Fatalf("expected json error")
		}
	})

	t.Run("bad endpoint", func(t *testing.T) {
		client := GoogleOAuthClient{TokenEndpoint: "http://%zz"}
		if _, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "refresh"}); err == nil {
			t.Fatalf("expected request construction error")
		}
	})

	t.Run("http client", func(t *testing.T) {
		client := GoogleOAuthClient{TokenEndpoint: "https://example.com/token", HTTP: errorHTTPClient{}}
		if _, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "refresh"}); err == nil || !strings.Contains(err.Error(), "http failed") {
			t.Fatalf("expected http error, got %v", err)
		}
	})

	t.Run("body read", func(t *testing.T) {
		client := GoogleOAuthClient{TokenEndpoint: "https://example.com/token", HTTP: staticHTTPClient{
			resp: &http.Response{StatusCode: http.StatusOK, Body: errorReadCloser{}},
		}}
		if _, err := client.Refresh(context.Background(), RefreshRequest{ClientID: "client-id", RefreshToken: "refresh"}); err == nil || !strings.Contains(err.Error(), "read failed") {
			t.Fatalf("expected body read error, got %v", err)
		}
	})
}

func TestGoogleOAuthClientRevokeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token", http.StatusBadRequest)
	}))
	defer server.Close()

	client := GoogleOAuthClient{RevocationEndpoint: server.URL}
	err := client.Revoke(context.Background(), "refresh")
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected revoke status error, got %v", err)
	}

	if err := (GoogleOAuthClient{RevocationEndpoint: "http://%zz"}).Revoke(context.Background(), "refresh"); err == nil {
		t.Fatalf("expected bad revoke endpoint error")
	}
	if err := (GoogleOAuthClient{HTTP: errorHTTPClient{}}).Revoke(context.Background(), "refresh"); err == nil || !strings.Contains(err.Error(), "http failed") {
		t.Fatalf("expected revoke http error, got %v", err)
	}
}

type errorHTTPClient struct{}

func (errorHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("http failed")
}

type staticHTTPClient struct {
	resp *http.Response
}

func (c staticHTTPClient) Do(*http.Request) (*http.Response, error) {
	return c.resp, nil
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errorReadCloser) Close() error {
	return nil
}

var _ io.ReadCloser = errorReadCloser{}
