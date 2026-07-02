package authsession

import (
	"fmt"
	"net/url"
	"strings"
)

// CallbackResult contains the validated authorization code callback data.
type CallbackResult struct {
	Code string
}

// ValidateCallback enforces the loopback callback checks from
// docs/architecture/cli-auth-flow.md before any token endpoint exchange.
func ValidateCallback(values url.Values, expectedState string) (CallbackResult, error) {
	if strings.TrimSpace(values.Get("state")) != strings.TrimSpace(expectedState) {
		return CallbackResult{}, fmt.Errorf("OAuth callback state mismatch")
	}
	if oauthErr := strings.TrimSpace(values.Get("error")); oauthErr != "" {
		desc := strings.TrimSpace(values.Get("error_description"))
		if desc != "" {
			return CallbackResult{}, fmt.Errorf("Google authorization failed: %s: %s", oauthErr, desc)
		}
		return CallbackResult{}, fmt.Errorf("Google authorization failed: %s", oauthErr)
	}
	code := strings.TrimSpace(values.Get("code"))
	if code == "" {
		return CallbackResult{}, fmt.Errorf("OAuth callback missing code")
	}
	return CallbackResult{Code: code}, nil
}
