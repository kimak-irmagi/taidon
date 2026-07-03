package authsession

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestResolveBearerTokenUsesEnvOverrideBeforeStoredSession(t *testing.T) {
	store := newMemoryCredentialStore()
	key := testCredentialKey()
	if err := store.Put(context.Background(), key, Session{CachedIDToken: "stored", IDTokenExpiry: time.Now().Add(time.Hour), RefreshToken: "refresh"}); err != nil {
		t.Fatalf("put session: %v", err)
	}
	t.Setenv("SQLRS_TOKEN", "env-token")

	manager := NewManager(ManagerOptions{Store: store, OAuth: &fakeOAuthClient{}, Clock: fixedClock{now: time.Now()}})
	got, err := manager.ResolveBearerToken(context.Background(), ResolveOptions{
		ProfileName: key.ProfileName,
		Endpoint:    key.Endpoint,
		AuthMode:    "oidcSession",
		ClientID:    key.ClientID,
		Issuer:      key.Issuer,
		TokenEnv:    "SQLRS_TOKEN",
	})
	if err != nil {
		t.Fatalf("ResolveBearerToken: %v", err)
	}
	if got.Token != "env-token" || got.Source != "env:SQLRS_TOKEN" {
		t.Fatalf("resolved = %+v, want env token", got)
	}
}

func TestResolveBearerTokenUsesFreshCachedIDToken(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "")
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newMemoryCredentialStore()
	key := testCredentialKey()
	session := Session{
		Provider:      "google",
		Issuer:        key.Issuer,
		ClientID:      key.ClientID,
		Email:         "alice@example.com",
		RefreshToken:  "refresh",
		CachedIDToken: "cached-id-token",
		IDTokenExpiry: now.Add(time.Hour),
	}
	if err := store.Put(context.Background(), key, session); err != nil {
		t.Fatalf("put session: %v", err)
	}

	manager := NewManager(ManagerOptions{Store: store, OAuth: &fakeOAuthClient{}, Clock: fixedClock{now: now}})
	got, err := manager.ResolveBearerToken(context.Background(), testResolveOptions())
	if err != nil {
		t.Fatalf("ResolveBearerToken: %v", err)
	}
	if got.Token != "cached-id-token" || got.Source != "stored_session" {
		t.Fatalf("resolved = %+v, want cached token", got)
	}
}

func TestResolveBearerTokenRefreshesExpiringSessionAndStoresIDToken(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "")
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newMemoryCredentialStore()
	key := testCredentialKey()
	if err := store.Put(context.Background(), key, Session{
		Provider:      "google",
		Issuer:        key.Issuer,
		ClientID:      key.ClientID,
		RefreshToken:  "refresh-old",
		CachedIDToken: "old-id-token",
		IDTokenExpiry: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("put session: %v", err)
	}
	newIDToken := testIDToken(t, map[string]any{
		"iss":   key.Issuer,
		"aud":   key.ClientID,
		"sub":   "subject-1",
		"email": "alice@example.com",
		"exp":   now.Add(time.Hour).Unix(),
	})
	oauth := &fakeOAuthClient{refreshResponse: TokenResponse{IDToken: newIDToken}}

	manager := NewManager(ManagerOptions{Store: store, OAuth: oauth, Clock: fixedClock{now: now}})
	got, err := manager.ResolveBearerToken(context.Background(), testResolveOptions())
	if err != nil {
		t.Fatalf("ResolveBearerToken: %v", err)
	}
	if got.Token != newIDToken || got.Source != "refreshed_session" {
		t.Fatalf("resolved = %+v, want refreshed token", got)
	}
	if oauth.refreshToken != "refresh-old" {
		t.Fatalf("refresh token = %q, want refresh-old", oauth.refreshToken)
	}
	if oauth.refreshClientSecret != "client-secret" {
		t.Fatalf("refresh client secret = %q, want client-secret", oauth.refreshClientSecret)
	}
	stored, ok, err := store.Get(context.Background(), key)
	if err != nil || !ok {
		t.Fatalf("get stored session: ok=%v err=%v", ok, err)
	}
	if stored.CachedIDToken != newIDToken || !stored.IDTokenExpiry.Equal(now.Add(time.Hour)) {
		t.Fatalf("stored session = %+v, want refreshed token metadata", stored)
	}
}

func TestResolveBearerTokenDeletesSessionOnRefreshFailure(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "")
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	store := newMemoryCredentialStore()
	key := testCredentialKey()
	if err := store.Put(context.Background(), key, Session{
		Provider:      "google",
		Issuer:        key.Issuer,
		ClientID:      key.ClientID,
		RefreshToken:  "refresh-old",
		IDTokenExpiry: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("put session: %v", err)
	}
	manager := NewManager(ManagerOptions{
		Store: store,
		OAuth: &fakeOAuthClient{refreshErr: errors.New("invalid_grant")},
		Clock: fixedClock{now: now},
	})

	_, err := manager.ResolveBearerToken(context.Background(), testResolveOptions())
	if err == nil || !strings.Contains(err.Error(), "auth login google") {
		t.Fatalf("expected login guidance, got %v", err)
	}
	if _, ok, err := store.Get(context.Background(), key); err != nil || ok {
		t.Fatalf("session should be deleted after refresh failure: ok=%v err=%v", ok, err)
	}
}

func TestLoginGoogleStoresRefreshTokenAndSafeMetadata(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	entropy := make([]byte, pkceEntropyBytes*3)
	for i := range entropy {
		entropy[i] = byte(i)
	}
	state := base64URL(entropy[pkceEntropyBytes : pkceEntropyBytes*2])
	nonce := base64URL(entropy[pkceEntropyBytes*2 : pkceEntropyBytes*3])
	key := testCredentialKey()
	idToken := testIDToken(t, map[string]any{
		"iss":   key.Issuer,
		"aud":   key.ClientID,
		"sub":   "subject-1",
		"email": "alice@example.com",
		"exp":   now.Add(time.Hour).Unix(),
		"nonce": nonce,
	})
	store := newMemoryCredentialStore()
	oauth := &fakeOAuthClient{exchangeResponse: TokenResponse{IDToken: idToken, RefreshToken: "refresh-token"}}
	loopback := fakeLoopbackProvider{session: &fakeLoopbackSession{
		redirectURI: "http://127.0.0.1:49152",
		callback:    url.Values{"state": {state}, "code": {"code-1"}},
	}}
	var readyURL string
	loopback.session.onWait = func() {
		if readyURL == "" {
			t.Fatalf("authorization URL was not reported before waiting for callback")
		}
	}
	manager := NewManager(ManagerOptions{
		Store:    store,
		OAuth:    oauth,
		Clock:    fixedClock{now: now},
		Rand:     bytes.NewReader(entropy),
		Loopback: loopback,
	})

	result, err := manager.LoginGoogle(context.Background(), LoginOptions{
		ProfileName:  key.ProfileName,
		Endpoint:     key.Endpoint,
		ClientID:     key.ClientID,
		ClientSecret: "client-secret",
		Issuer:       key.Issuer,
		NoBrowser:    true,
		AuthorizationURLReady: func(authURL string) error {
			readyURL = authURL
			return nil
		},
	})
	if err != nil {
		t.Fatalf("LoginGoogle: %v", err)
	}
	if result.Email != "alice@example.com" || result.Provider != "google" {
		t.Fatalf("result = %+v, want safe metadata", result)
	}
	if result.AuthorizationURL == "" {
		t.Fatalf("no-browser login should return authorization URL")
	}
	if readyURL != result.AuthorizationURL {
		t.Fatalf("reported URL = %q, result URL = %q", readyURL, result.AuthorizationURL)
	}
	if oauth.exchangeCode != "code-1" {
		t.Fatalf("exchange code = %q, want code-1", oauth.exchangeCode)
	}
	if oauth.exchangeClientSecret != "client-secret" {
		t.Fatalf("exchange client secret = %q, want client-secret", oauth.exchangeClientSecret)
	}
	stored, ok, err := store.Get(context.Background(), key)
	if err != nil || !ok {
		t.Fatalf("get session: ok=%v err=%v", ok, err)
	}
	if stored.RefreshToken != "refresh-token" || stored.CachedIDToken != idToken {
		t.Fatalf("stored token metadata = %+v", stored)
	}
}

func TestLoginGoogleRequiresRefreshToken(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	entropy := make([]byte, pkceEntropyBytes*3)
	state := base64URL(entropy[pkceEntropyBytes : pkceEntropyBytes*2])
	nonce := base64URL(entropy[pkceEntropyBytes*2 : pkceEntropyBytes*3])
	key := testCredentialKey()
	idToken := testIDToken(t, map[string]any{
		"iss":   key.Issuer,
		"aud":   key.ClientID,
		"sub":   "subject-1",
		"exp":   now.Add(time.Hour).Unix(),
		"nonce": nonce,
	})
	manager := NewManager(ManagerOptions{
		Store: newMemoryCredentialStore(),
		OAuth: &fakeOAuthClient{exchangeResponse: TokenResponse{IDToken: idToken}},
		Clock: fixedClock{now: now},
		Rand:  bytes.NewReader(entropy),
		Loopback: fakeLoopbackProvider{session: &fakeLoopbackSession{
			redirectURI: "http://127.0.0.1:49152",
			callback:    url.Values{"state": {state}, "code": {"code-1"}},
		}},
	})

	_, err := manager.LoginGoogle(context.Background(), LoginOptions{
		ProfileName: key.ProfileName,
		Endpoint:    key.Endpoint,
		ClientID:    key.ClientID,
		Issuer:      key.Issuer,
		NoBrowser:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "refresh_token") {
		t.Fatalf("expected refresh_token error, got %v", err)
	}
}

func TestStatusReportsStoredSessionAndEnvOverride(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "")
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	key := testCredentialKey()
	store := newMemoryCredentialStore()
	if err := store.Put(context.Background(), key, Session{
		Provider:      "google",
		Issuer:        key.Issuer,
		ClientID:      key.ClientID,
		Email:         "alice@example.com",
		Subject:       "subject-1",
		IDTokenExpiry: now.Add(time.Hour),
		RefreshToken:  "refresh-token",
	}); err != nil {
		t.Fatalf("put session: %v", err)
	}
	manager := NewManager(ManagerOptions{Store: store, OAuth: &fakeOAuthClient{}, Clock: fixedClock{now: now}})

	status, err := manager.Status(context.Background(), StatusOptions{
		ProfileName: key.ProfileName,
		Endpoint:    key.Endpoint,
		AuthMode:    "oidcSession",
		ClientID:    key.ClientID,
		Issuer:      key.Issuer,
		TokenEnv:    "SQLRS_TOKEN",
	})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.LoggedIn || status.Email != "alice@example.com" || status.Override != "" {
		t.Fatalf("status = %+v, want stored logged-in session", status)
	}

	t.Setenv("SQLRS_TOKEN", "debug-token")
	status, err = manager.Status(context.Background(), StatusOptions{
		ProfileName: key.ProfileName,
		Endpoint:    key.Endpoint,
		AuthMode:    "oidcSession",
		ClientID:    key.ClientID,
		Issuer:      key.Issuer,
		TokenEnv:    "SQLRS_TOKEN",
	})
	if err != nil {
		t.Fatalf("Status with env: %v", err)
	}
	if !status.LoggedIn || status.Override != "SQLRS_TOKEN" {
		t.Fatalf("status = %+v, want SQLRS_TOKEN override", status)
	}
}

func TestLogoutRevokesThenDeletesStoredSession(t *testing.T) {
	key := testCredentialKey()
	store := newMemoryCredentialStore()
	if err := store.Put(context.Background(), key, Session{Provider: "google", RefreshToken: "refresh-token"}); err != nil {
		t.Fatalf("put session: %v", err)
	}
	oauth := &fakeOAuthClient{}
	manager := NewManager(ManagerOptions{Store: store, OAuth: oauth, Clock: fixedClock{now: time.Now()}})

	result, err := manager.Logout(context.Background(), LogoutOptions{
		ProfileName: key.ProfileName,
		Endpoint:    key.Endpoint,
		ClientID:    key.ClientID,
		Issuer:      key.Issuer,
	})
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !result.Revoked {
		t.Fatalf("result = %+v, want revoked", result)
	}
	if oauth.revokedToken != "refresh-token" {
		t.Fatalf("revoked token = %q, want refresh-token", oauth.revokedToken)
	}
	if _, ok, err := store.Get(context.Background(), key); err != nil || ok {
		t.Fatalf("session should be deleted: ok=%v err=%v", ok, err)
	}
}

func testResolveOptions() ResolveOptions {
	key := testCredentialKey()
	return ResolveOptions{
		ProfileName:  key.ProfileName,
		Endpoint:     key.Endpoint,
		AuthMode:     "oidcSession",
		ClientID:     key.ClientID,
		ClientSecret: "client-secret",
		Issuer:       key.Issuer,
		TokenEnv:     "SQLRS_TOKEN",
	}
}

func testCredentialKey() CredentialKey {
	return CredentialKey{
		ProfileName: "remote-dev",
		Endpoint:    "https://sqlrs.example.org",
		Provider:    "google",
		Issuer:      "https://accounts.google.com",
		ClientID:    "client-id",
	}
}

type fakeOAuthClient struct {
	exchangeResponse     TokenResponse
	exchangeErr          error
	exchangeCode         string
	exchangeClientSecret string

	refreshResponse     TokenResponse
	refreshErr          error
	refreshToken        string
	refreshClientSecret string

	revokeErr    error
	revokedToken string
}

func (f *fakeOAuthClient) ExchangeCode(_ context.Context, req CodeExchangeRequest) (TokenResponse, error) {
	f.exchangeCode = req.Code
	f.exchangeClientSecret = req.ClientSecret
	if f.exchangeErr != nil {
		return TokenResponse{}, f.exchangeErr
	}
	return f.exchangeResponse, nil
}

func (f *fakeOAuthClient) Refresh(_ context.Context, req RefreshRequest) (TokenResponse, error) {
	f.refreshToken = req.RefreshToken
	f.refreshClientSecret = req.ClientSecret
	if f.refreshErr != nil {
		return TokenResponse{}, f.refreshErr
	}
	return f.refreshResponse, nil
}

func (f *fakeOAuthClient) Revoke(_ context.Context, token string) error {
	f.revokedToken = token
	return f.revokeErr
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type fakeLoopbackProvider struct {
	session *fakeLoopbackSession
	err     error
}

func (p fakeLoopbackProvider) Start(context.Context) (LoopbackSession, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.session, nil
}

type fakeLoopbackSession struct {
	redirectURI string
	callback    url.Values
	closed      bool
	onWait      func()
}

func (s *fakeLoopbackSession) RedirectURI() string {
	return s.redirectURI
}

func (s *fakeLoopbackSession) Wait(context.Context) (url.Values, error) {
	if s.onWait != nil {
		s.onWait()
	}
	return s.callback, nil
}

func (s *fakeLoopbackSession) Close() error {
	s.closed = true
	return nil
}

func base64URL(data []byte) string {
	return strings.TrimRight(base64Raw(data), "=")
}

func base64Raw(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}
