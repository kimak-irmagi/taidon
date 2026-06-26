package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetCurrentUser reads the authenticated user's profile from the remote
// user/org API described in docs/architecture/user-org-component-structure.md.
func (c *Client) GetCurrentUser(ctx context.Context) (UserProfileResult, bool, error) {
	var out UserProfileResult
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/users/me", true)
	if err != nil {
		return out, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return out, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, false, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, false, err
	}
	c.captureEntityHeaders(resp, &out)
	return out, true, nil
}

// PutCurrentUserCreate creates the authenticated user profile with
// If-None-Match:* so callers can safely retry after transport failures.
func (c *Client) PutCurrentUserCreate(ctx context.Context, req UserProfileWriteRequest) (UserProfileResult, int, error) {
	return c.putUserProfile(ctx, "/v1/users/me", map[string]string{"If-None-Match": "*"}, req)
}

// PutCurrentUserUpdate updates the authenticated user profile with If-Match.
func (c *Client) PutCurrentUserUpdate(ctx context.Context, etag string, req UserProfileWriteRequest) (UserProfileResult, int, error) {
	return c.putUserProfile(ctx, "/v1/users/me", map[string]string{"If-Match": etag}, req)
}

func (c *Client) GetUserByIdentity(ctx context.Context, key IdentityKey) (UserProfileResult, bool, error) {
	var out UserProfileResult
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/users/by-identity"+identityQuery(key), true)
	if err != nil {
		return out, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return out, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, false, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, false, err
	}
	c.captureEntityHeaders(resp, &out)
	return out, true, nil
}

// PutUserByIdentityCreate provisions another user against the external identity
// natural key, using If-None-Match:* to prevent duplicate identity binding.
func (c *Client) PutUserByIdentityCreate(ctx context.Context, key IdentityKey, req UserProfileWriteRequest) (UserProfileResult, int, error) {
	return c.putUserProfile(ctx, "/v1/users/by-identity"+identityQuery(key), map[string]string{"If-None-Match": "*"}, req)
}

// PutUserByIdentityUpdate updates a user selected by external identity using If-Match.
func (c *Client) PutUserByIdentityUpdate(ctx context.Context, key IdentityKey, etag string, req UserProfileWriteRequest) (UserProfileResult, int, error) {
	return c.putUserProfile(ctx, "/v1/users/by-identity"+identityQuery(key), map[string]string{"If-Match": etag}, req)
}

func (c *Client) putUserProfile(ctx context.Context, path string, headers map[string]string, req UserProfileWriteRequest) (UserProfileResult, int, error) {
	var out UserProfileResult
	body, err := json.Marshal(req)
	if err != nil {
		return out, 0, err
	}
	resp, err := c.doRequestWithBodyHeaders(ctx, http.MethodPut, path, true, bytes.NewReader(body), "application/json", headers)
	if err != nil {
		return out, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return out, resp.StatusCode, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, resp.StatusCode, err
	}
	c.captureEntityHeaders(resp, &out)
	return out, resp.StatusCode, nil
}

func identityQuery(key IdentityKey) string {
	values := url.Values{}
	values.Set("provider", strings.TrimSpace(key.Provider))
	values.Set("issuer", strings.TrimSpace(key.Issuer))
	values.Set("subject", strings.TrimSpace(key.Subject))
	return "?" + values.Encode()
}

func (c *Client) captureEntityHeaders(resp *http.Response, out *UserProfileResult) {
	out.ETag = resp.Header.Get("ETag")
	out.Location = resp.Header.Get("Location")
}

// CreateOrganization creates an organization and returns the caller's initial
// admin membership, per docs/architecture/user-org-flow.md.
func (c *Client) CreateOrganization(ctx context.Context, req OrganizationCreateRequest) (OrganizationCreateResponse, int, error) {
	var out OrganizationCreateResponse
	body, err := json.Marshal(req)
	if err != nil {
		return out, 0, err
	}
	resp, err := c.doRequestWithBody(ctx, http.MethodPost, "/v1/organizations", true, bytes.NewReader(body), "application/json")
	if err != nil {
		return out, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return out, resp.StatusCode, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, resp.StatusCode, err
	}
	return out, resp.StatusCode, nil
}

func (c *Client) ListOrganizations(ctx context.Context) ([]OrganizationMembershipView, error) {
	var out []OrganizationMembershipView
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/organizations", true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []OrganizationMembershipView{}
	}
	return out, nil
}

func (c *Client) GetOrganization(ctx context.Context, orgRef string) (OrganizationMembershipView, bool, error) {
	var out OrganizationMembershipView
	ref := strings.TrimSpace(orgRef)
	if ref == "" {
		return out, false, fmt.Errorf("organization reference is required")
	}
	resp, err := c.doRequest(ctx, http.MethodGet, "/v1/organizations/"+url.PathEscape(ref), true)
	if err != nil {
		return out, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return out, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, false, parseErrorResponse(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, false, err
	}
	return out, true, nil
}
