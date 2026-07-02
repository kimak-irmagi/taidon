package authsession

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint         = "https://oauth2.googleapis.com/token"
	googleRevocationEndpoint    = "https://oauth2.googleapis.com/revoke"
	defaultGoogleIssuer         = "https://accounts.google.com"
)

var googleOIDCScopes = []string{"openid", "email", "profile"}

// AuthURLOptions are the non-secret inputs used to construct the Google
// authorization URL for the flow documented in docs/architecture/cli-auth-flow.md.
type AuthURLOptions struct {
	ClientID              string
	RedirectURI           string
	State                 string
	Nonce                 string
	CodeChallenge         string
	LoginHint             string
	AuthorizationEndpoint string
	Scopes                []string
}

// BuildGoogleAuthURL builds the Google OIDC authorization URL with PKCE,
// offline access, and consent prompting so Google can issue a refresh token.
func BuildGoogleAuthURL(opts AuthURLOptions) (string, error) {
	endpoint := strings.TrimSpace(opts.AuthorizationEndpoint)
	if endpoint == "" {
		endpoint = googleAuthorizationEndpoint
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return "", fmt.Errorf("auth.clientID is required")
	}
	if strings.TrimSpace(opts.RedirectURI) == "" {
		return "", fmt.Errorf("redirect URI is required")
	}
	if strings.TrimSpace(opts.State) == "" {
		return "", fmt.Errorf("state is required")
	}
	if strings.TrimSpace(opts.Nonce) == "" {
		return "", fmt.Errorf("nonce is required")
	}
	if strings.TrimSpace(opts.CodeChallenge) == "" {
		return "", fmt.Errorf("code challenge is required")
	}
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = googleOIDCScopes
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", strings.TrimSpace(opts.ClientID))
	query.Set("redirect_uri", strings.TrimSpace(opts.RedirectURI))
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", strings.TrimSpace(opts.State))
	query.Set("nonce", strings.TrimSpace(opts.Nonce))
	query.Set("code_challenge", strings.TrimSpace(opts.CodeChallenge))
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	if strings.TrimSpace(opts.LoginHint) != "" {
		query.Set("login_hint", strings.TrimSpace(opts.LoginHint))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// CodeExchangeRequest is the OAuth token endpoint request for a loopback
// authorization code and PKCE verifier.
type CodeExchangeRequest struct {
	ClientID     string
	Code         string
	CodeVerifier string
	RedirectURI  string
}

// RefreshRequest is the OAuth refresh-token grant request. Refresh tokens are
// sent only to Google, never to the sqlrs gateway.
type RefreshRequest struct {
	ClientID     string
	RefreshToken string
}

// TokenResponse is the subset of Google token endpoint fields used by the CLI.
type TokenResponse struct {
	IDToken      string
	RefreshToken string
	AccessToken  string
	ExpiresIn    int
	TokenType    string
	Scope        string
}

// OAuthClient abstracts Google token and revocation endpoint calls for tests.
type OAuthClient interface {
	ExchangeCode(context.Context, CodeExchangeRequest) (TokenResponse, error)
	Refresh(context.Context, RefreshRequest) (TokenResponse, error)
	Revoke(context.Context, string) error
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// GoogleOAuthClient calls Google's OAuth token and revocation endpoints.
type GoogleOAuthClient struct {
	HTTP               HTTPClient
	TokenEndpoint      string
	RevocationEndpoint string
}

func (c GoogleOAuthClient) ExchangeCode(ctx context.Context, req CodeExchangeRequest) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(req.ClientID))
	form.Set("code", strings.TrimSpace(req.Code))
	form.Set("code_verifier", strings.TrimSpace(req.CodeVerifier))
	form.Set("redirect_uri", strings.TrimSpace(req.RedirectURI))
	form.Set("grant_type", "authorization_code")
	return c.postToken(ctx, form)
}

func (c GoogleOAuthClient) Refresh(ctx context.Context, req RefreshRequest) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(req.ClientID))
	form.Set("refresh_token", strings.TrimSpace(req.RefreshToken))
	form.Set("grant_type", "refresh_token")
	return c.postToken(ctx, form)
}

func (c GoogleOAuthClient) Revoke(ctx context.Context, token string) error {
	endpoint := strings.TrimSpace(c.RevocationEndpoint)
	if endpoint == "" {
		endpoint = googleRevocationEndpoint
	}
	form := url.Values{"token": {strings.TrimSpace(token)}}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Google token revocation failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (c GoogleOAuthClient) postToken(ctx context.Context, form url.Values) (TokenResponse, error) {
	endpoint := strings.TrimSpace(c.TokenEndpoint)
	if endpoint == "" {
		endpoint = googleTokenEndpoint
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResponse{}, fmt.Errorf("Google token endpoint failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var raw struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		IDToken:      raw.IDToken,
		RefreshToken: raw.RefreshToken,
		AccessToken:  raw.AccessToken,
		ExpiresIn:    raw.ExpiresIn,
		TokenType:    raw.TokenType,
		Scope:        raw.Scope,
	}, nil
}

func defaultIssuer(issuer string) string {
	if strings.TrimSpace(issuer) == "" {
		return defaultGoogleIssuer
	}
	return strings.TrimSpace(issuer)
}
