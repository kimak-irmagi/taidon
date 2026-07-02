package authsession

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultRefreshWindow = 5 * time.Minute

var ErrLoginRequired = errors.New("Google auth session is expired or unavailable; run `sqlrs auth login google`")

// Clock is injected so refresh decisions can be tested deterministically.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

// BrowserOpener opens the authorization URL in the user's browser.
type BrowserOpener func(context.Context, string) error

// LoopbackProvider creates the temporary 127.0.0.1 listener used by login.
type LoopbackProvider interface {
	Start(context.Context) (LoopbackSession, error)
}

// LoopbackSession represents one authorization callback listener.
type LoopbackSession interface {
	RedirectURI() string
	Wait(context.Context) (url.Values, error)
	Close() error
}

// ManagerOptions wires the auth session manager dependencies.
type ManagerOptions struct {
	Store       CredentialStore
	OAuth       OAuthClient
	Clock       Clock
	Rand        io.Reader
	Loopback    LoopbackProvider
	OpenBrowser BrowserOpener
}

// Manager owns Google OIDC login, status, logout, and bearer-token resolution
// for CLI profiles, per docs/architecture/cli-auth-component-structure.md.
type Manager struct {
	store       CredentialStore
	oauth       OAuthClient
	clock       Clock
	rand        io.Reader
	loopback    LoopbackProvider
	openBrowser BrowserOpener
}

func NewManager(opts ManagerOptions) *Manager {
	store := opts.Store
	if store == nil {
		store = NewSystemCredentialStore()
	}
	oauth := opts.OAuth
	if oauth == nil {
		oauth = GoogleOAuthClient{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}
	random := opts.Rand
	if random == nil {
		random = rand.Reader
	}
	loopback := opts.Loopback
	if loopback == nil {
		loopback = LoopbackServerProvider{}
	}
	openBrowser := opts.OpenBrowser
	if openBrowser == nil {
		openBrowser = OpenBrowser
	}
	return &Manager{store: store, oauth: oauth, clock: clock, rand: random, loopback: loopback, openBrowser: openBrowser}
}

type LoginOptions struct {
	ProfileName string
	Endpoint    string
	ClientID    string
	Issuer      string
	LoginHint   string
	NoBrowser   bool
}

type LoginResult struct {
	LoggedIn         bool      `json:"logged_in"`
	Provider         string    `json:"provider"`
	Email            string    `json:"email,omitempty"`
	Issuer           string    `json:"issuer"`
	Audience         string    `json:"audience"`
	Subject          string    `json:"subject,omitempty"`
	TokenExpiry      time.Time `json:"token_expiry,omitempty"`
	Profile          string    `json:"profile"`
	Endpoint         string    `json:"endpoint"`
	AuthorizationURL string    `json:"authorization_url,omitempty"`
}

func (m *Manager) LoginGoogle(ctx context.Context, opts LoginOptions) (LoginResult, error) {
	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		return LoginResult{}, fmt.Errorf("auth.clientID is required for sqlrs auth login google")
	}
	issuer := defaultIssuer(opts.Issuer)
	pair, err := GeneratePKCEPair(m.rand)
	if err != nil {
		return LoginResult{}, err
	}
	state, err := generateOpaqueToken(m.rand)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	nonce, err := generateOpaqueToken(m.rand)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate OIDC nonce: %w", err)
	}
	listener, err := m.loopback.Start(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	defer listener.Close()

	authURL, err := BuildGoogleAuthURL(AuthURLOptions{
		ClientID:      clientID,
		RedirectURI:   listener.RedirectURI(),
		State:         state,
		Nonce:         nonce,
		CodeChallenge: pair.Challenge,
		LoginHint:     opts.LoginHint,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if !opts.NoBrowser {
		if err := m.openBrowser(ctx, authURL); err != nil {
			return LoginResult{}, err
		}
	}
	callbackValues, err := listener.Wait(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	callback, err := ValidateCallback(callbackValues, state)
	if err != nil {
		return LoginResult{}, err
	}
	resp, err := m.oauth.ExchangeCode(ctx, CodeExchangeRequest{
		ClientID:     clientID,
		Code:         callback.Code,
		CodeVerifier: pair.Verifier,
		RedirectURI:  listener.RedirectURI(),
	})
	if err != nil {
		return LoginResult{}, err
	}
	if strings.TrimSpace(resp.IDToken) == "" {
		return LoginResult{}, fmt.Errorf("Google token response missing id_token")
	}
	if strings.TrimSpace(resp.RefreshToken) == "" {
		return LoginResult{}, fmt.Errorf("Google token response missing refresh_token; retry `sqlrs auth login google` and check offline consent")
	}
	claims, err := DecodeIDTokenClaims(resp.IDToken)
	if err != nil {
		return LoginResult{}, err
	}
	if err := ValidateIDTokenClaims(claims, issuer, clientID, nonce, m.clock.Now()); err != nil {
		return LoginResult{}, err
	}
	now := m.clock.Now().UTC()
	session := Session{
		Provider:      "google",
		Issuer:        issuer,
		ClientID:      clientID,
		Subject:       claims.Subject,
		Email:         claims.Email,
		Scopes:        append([]string(nil), googleOIDCScopes...),
		RefreshToken:  strings.TrimSpace(resp.RefreshToken),
		CachedIDToken: strings.TrimSpace(resp.IDToken),
		IDTokenExpiry: claims.Expiry,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	key := credentialKey(opts.ProfileName, opts.Endpoint, issuer, clientID)
	if err := m.store.Put(ctx, key, session); err != nil {
		return LoginResult{}, err
	}
	result := loginResultFromSession(opts.ProfileName, opts.Endpoint, session)
	result.LoggedIn = true
	if opts.NoBrowser {
		result.AuthorizationURL = authURL
	}
	return result, nil
}

type StatusOptions struct {
	ProfileName string
	Endpoint    string
	AuthMode    string
	ClientID    string
	Issuer      string
	TokenEnv    string
}

type StatusResult struct {
	LoggedIn    bool      `json:"logged_in"`
	Provider    string    `json:"provider"`
	Email       string    `json:"email,omitempty"`
	Issuer      string    `json:"issuer"`
	Audience    string    `json:"audience"`
	Subject     string    `json:"subject,omitempty"`
	TokenExpiry time.Time `json:"token_expiry,omitempty"`
	Profile     string    `json:"profile"`
	Endpoint    string    `json:"endpoint"`
	Override    string    `json:"override,omitempty"`
}

func (m *Manager) Status(ctx context.Context, opts StatusOptions) (StatusResult, error) {
	overrideEnv := configuredTokenEnv(opts.TokenEnv, opts.AuthMode)
	result := StatusResult{
		Profile:  strings.TrimSpace(opts.ProfileName),
		Endpoint: strings.TrimSpace(opts.Endpoint),
		Issuer:   defaultIssuer(opts.Issuer),
		Audience: strings.TrimSpace(opts.ClientID),
		Provider: "google",
	}
	if overrideEnv != "" && strings.TrimSpace(os.Getenv(overrideEnv)) != "" {
		result.LoggedIn = true
		result.Override = overrideEnv
		return result, nil
	}
	if !strings.EqualFold(strings.TrimSpace(opts.AuthMode), "oidcSession") {
		return result, nil
	}
	session, ok, err := m.store.Get(ctx, credentialKey(opts.ProfileName, opts.Endpoint, opts.Issuer, opts.ClientID))
	if err != nil {
		return StatusResult{}, err
	}
	if !ok {
		return result, nil
	}
	return statusResultFromSession(opts.ProfileName, opts.Endpoint, session), nil
}

type LogoutOptions struct {
	ProfileName string
	Endpoint    string
	ClientID    string
	Issuer      string
	NoRevoke    bool
}

type LogoutResult struct {
	Provider         string `json:"provider"`
	Profile          string `json:"profile"`
	Endpoint         string `json:"endpoint"`
	Deleted          bool   `json:"deleted"`
	Revoked          bool   `json:"revoked"`
	RevocationFailed string `json:"revocation_failed,omitempty"`
}

func (m *Manager) Logout(ctx context.Context, opts LogoutOptions) (LogoutResult, error) {
	key := credentialKey(opts.ProfileName, opts.Endpoint, opts.Issuer, opts.ClientID)
	session, ok, err := m.store.Get(ctx, key)
	if err != nil {
		return LogoutResult{}, err
	}
	result := LogoutResult{
		Provider: "google",
		Profile:  strings.TrimSpace(opts.ProfileName),
		Endpoint: strings.TrimSpace(opts.Endpoint),
	}
	if ok && !opts.NoRevoke && strings.TrimSpace(session.RefreshToken) != "" {
		if err := m.oauth.Revoke(ctx, session.RefreshToken); err != nil {
			result.RevocationFailed = err.Error()
		} else {
			result.Revoked = true
		}
	}
	if err := m.store.Delete(ctx, key); err != nil {
		return result, err
	}
	result.Deleted = true
	return result, nil
}

type ResolveOptions struct {
	ProfileName   string
	Endpoint      string
	AuthMode      string
	ClientID      string
	Issuer        string
	TokenEnv      string
	StaticToken   string
	RefreshWindow time.Duration
}

type ResolvedBearerToken struct {
	Token  string
	Source string
}

func (m *Manager) ResolveBearerToken(ctx context.Context, opts ResolveOptions) (ResolvedBearerToken, error) {
	tokenEnv := configuredTokenEnv(opts.TokenEnv, opts.AuthMode)
	if tokenEnv != "" {
		if value := strings.TrimSpace(os.Getenv(tokenEnv)); value != "" {
			return ResolvedBearerToken{Token: value, Source: "env:" + tokenEnv}, nil
		}
	}
	if !strings.EqualFold(strings.TrimSpace(opts.AuthMode), "oidcSession") {
		if token := strings.TrimSpace(opts.StaticToken); token != "" {
			return ResolvedBearerToken{Token: token, Source: "static_token"}, nil
		}
		return ResolvedBearerToken{}, nil
	}
	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		return ResolvedBearerToken{}, fmt.Errorf("auth.clientID is required for oidcSession profiles")
	}
	key := credentialKey(opts.ProfileName, opts.Endpoint, opts.Issuer, clientID)
	session, ok, err := m.store.Get(ctx, key)
	if err != nil {
		return ResolvedBearerToken{}, err
	}
	if !ok || strings.TrimSpace(session.RefreshToken) == "" {
		return ResolvedBearerToken{}, ErrLoginRequired
	}
	window := opts.RefreshWindow
	if window == 0 {
		window = defaultRefreshWindow
	}
	now := m.clock.Now().UTC()
	if strings.TrimSpace(session.CachedIDToken) != "" && !ShouldRefresh(session.IDTokenExpiry, now, window) {
		return ResolvedBearerToken{Token: session.CachedIDToken, Source: "stored_session"}, nil
	}
	resp, err := m.oauth.Refresh(ctx, RefreshRequest{ClientID: clientID, RefreshToken: session.RefreshToken})
	if err != nil {
		_ = m.store.Delete(ctx, key)
		return ResolvedBearerToken{}, fmt.Errorf("%w: %v", ErrLoginRequired, err)
	}
	if strings.TrimSpace(resp.IDToken) == "" {
		_ = m.store.Delete(ctx, key)
		return ResolvedBearerToken{}, fmt.Errorf("%w: Google token response missing id_token", ErrLoginRequired)
	}
	claims, err := DecodeIDTokenClaims(resp.IDToken)
	if err != nil {
		_ = m.store.Delete(ctx, key)
		return ResolvedBearerToken{}, fmt.Errorf("%w: %v", ErrLoginRequired, err)
	}
	if err := ValidateIDTokenClaims(claims, defaultIssuer(opts.Issuer), clientID, "", now); err != nil {
		_ = m.store.Delete(ctx, key)
		return ResolvedBearerToken{}, fmt.Errorf("%w: %v", ErrLoginRequired, err)
	}
	if strings.TrimSpace(resp.RefreshToken) != "" {
		session.RefreshToken = strings.TrimSpace(resp.RefreshToken)
	}
	session.Provider = "google"
	session.Issuer = defaultIssuer(opts.Issuer)
	session.ClientID = clientID
	session.Subject = claims.Subject
	session.Email = claims.Email
	session.CachedIDToken = strings.TrimSpace(resp.IDToken)
	session.IDTokenExpiry = claims.Expiry
	session.UpdatedAt = now
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if err := m.store.Put(ctx, key, session); err != nil {
		return ResolvedBearerToken{}, err
	}
	return ResolvedBearerToken{Token: session.CachedIDToken, Source: "refreshed_session"}, nil
}

func credentialKey(profileName, endpoint, issuer, clientID string) CredentialKey {
	return CredentialKey{
		ProfileName: profileName,
		Endpoint:    endpoint,
		Provider:    "google",
		Issuer:      defaultIssuer(issuer),
		ClientID:    strings.TrimSpace(clientID),
	}
}

func configuredTokenEnv(tokenEnv string, authMode string) string {
	tokenEnv = strings.TrimSpace(tokenEnv)
	if tokenEnv == "" && strings.EqualFold(strings.TrimSpace(authMode), "oidcSession") {
		return "SQLRS_TOKEN"
	}
	return tokenEnv
}

func loginResultFromSession(profileName, endpoint string, session Session) LoginResult {
	return LoginResult{
		LoggedIn:    true,
		Provider:    session.Provider,
		Email:       session.Email,
		Issuer:      session.Issuer,
		Audience:    session.ClientID,
		Subject:     maskSubject(session.Subject),
		TokenExpiry: session.IDTokenExpiry,
		Profile:     strings.TrimSpace(profileName),
		Endpoint:    strings.TrimSpace(endpoint),
	}
}

func statusResultFromSession(profileName, endpoint string, session Session) StatusResult {
	return StatusResult{
		LoggedIn:    true,
		Provider:    session.Provider,
		Email:       session.Email,
		Issuer:      session.Issuer,
		Audience:    session.ClientID,
		Subject:     maskSubject(session.Subject),
		TokenExpiry: session.IDTokenExpiry,
		Profile:     strings.TrimSpace(profileName),
		Endpoint:    strings.TrimSpace(endpoint),
	}
}
