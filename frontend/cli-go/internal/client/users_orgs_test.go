package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUserProfileHTTPMethodsUseConditionalPUT(t *testing.T) {
	var sawGetMe bool
	var sawPutMe bool
	var sawPutIdentity bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer token", auth)
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/me":
			sawGetMe = true
			w.Header().Set("ETag", `"user-etag"`)
			_, _ = w.Write([]byte(`{"status":"existing","user":{"id":"usr_1","display_name":"Evil Guest","email":"evil@example.test"},"identities":[{"provider":"oidc","issuer":"https://issuer.example.test","subject":"sub-1"}],"memberships":[]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/users/me":
			sawPutMe = true
			if got := r.Header.Get("If-None-Match"); got != "*" {
				t.Fatalf("If-None-Match = %q, want *", got)
			}
			var req UserProfileWriteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode current user request: %v", err)
			}
			if req.DisplayName != "Evil Guest" || req.Email == nil || *req.Email != "evil@example.test" {
				t.Fatalf("unexpected current user request: %+v", req)
			}
			w.Header().Set("ETag", `"created-etag"`)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"created","user":{"id":"usr_1","display_name":"Evil Guest","email":"evil@example.test"},"identities":[],"memberships":[]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/users/by-identity":
			sawPutIdentity = true
			if got := r.Header.Get("If-None-Match"); got != "*" {
				t.Fatalf("identity If-None-Match = %q, want *", got)
			}
			query := r.URL.Query()
			if query.Get("provider") != "oidc" || query.Get("issuer") != "https://issuer.example.test" || query.Get("subject") != "sub/2" {
				t.Fatalf("unexpected identity query: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"created","user":{"id":"usr_2","display_name":"New User"},"identities":[{"provider":"oidc","issuer":"https://issuer.example.test","subject":"sub/2"}],"memberships":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	email := "evil@example.test"
	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "secret"})

	me, found, err := cli.GetCurrentUser(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if !found || me.ETag != `"user-etag"` || me.User.ID != "usr_1" {
		t.Fatalf("unexpected current user result: found=%v result=%+v", found, me)
	}

	created, status, err := cli.PutCurrentUserCreate(context.Background(), UserProfileWriteRequest{
		DisplayName: "Evil Guest",
		Email:       &email,
	})
	if err != nil {
		t.Fatalf("PutCurrentUserCreate: %v", err)
	}
	if status != http.StatusCreated || created.ETag != `"created-etag"` || created.Status != "created" {
		t.Fatalf("unexpected create result: status=%d result=%+v", status, created)
	}

	provisioned, status, err := cli.PutUserByIdentityCreate(context.Background(), IdentityKey{
		Provider: "oidc",
		Issuer:   "https://issuer.example.test",
		Subject:  "sub/2",
	}, UserProfileWriteRequest{DisplayName: "New User"})
	if err != nil {
		t.Fatalf("PutUserByIdentityCreate: %v", err)
	}
	if status != http.StatusCreated || provisioned.User.ID != "usr_2" {
		t.Fatalf("unexpected provision result: status=%d result=%+v", status, provisioned)
	}

	if !sawGetMe || !sawPutMe || !sawPutIdentity {
		t.Fatalf("expected all user endpoints to be exercised: get=%v putMe=%v putIdentity=%v", sawGetMe, sawPutMe, sawPutIdentity)
	}
}

func TestUserProfileUpdateUsesIfMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("If-Match"); got != `"known-etag"` {
			t.Fatalf("If-Match = %q, want known etag", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"updated","user":{"id":"usr_1","display_name":"Updated"},"identities":[],"memberships":[]}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	result, status, err := cli.PutCurrentUserUpdate(context.Background(), `"known-etag"`, UserProfileWriteRequest{DisplayName: "Updated"})
	if err != nil {
		t.Fatalf("PutCurrentUserUpdate: %v", err)
	}
	if status != http.StatusOK || result.Status != "updated" {
		t.Fatalf("unexpected update result: status=%d result=%+v", status, result)
	}
}

func TestOrganizationHTTPMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/organizations":
			var req OrganizationCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode org create request: %v", err)
			}
			if req.Slug != "evil-lab" || req.DisplayName != "Evil Lab" {
				t.Fatalf("unexpected org create request: %+v", req)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"organization":{"id":"org_1","slug":"evil-lab","display_name":"Evil Lab"},"membership":{"user_id":"usr_1","organization_id":"org_1","role":"admin"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations":
			_, _ = w.Write([]byte(`[{"organization":{"id":"org_1","slug":"evil-lab","display_name":"Evil Lab"},"membership":{"user_id":"usr_1","organization_id":"org_1","role":"admin"}}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations/evil-lab":
			_, _ = w.Write([]byte(`{"organization":{"id":"org_1","slug":"evil-lab","display_name":"Evil Lab"},"membership":{"user_id":"usr_1","organization_id":"org_1","role":"admin"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})

	created, status, err := cli.CreateOrganization(context.Background(), OrganizationCreateRequest{Slug: "evil-lab", DisplayName: "Evil Lab"})
	if err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	if status != http.StatusCreated || created.Membership.Role != "admin" {
		t.Fatalf("unexpected create result: status=%d result=%+v", status, created)
	}

	list, err := cli.ListOrganizations(context.Background())
	if err != nil {
		t.Fatalf("ListOrganizations: %v", err)
	}
	if len(list) != 1 || list[0].Organization.Slug != "evil-lab" {
		t.Fatalf("unexpected org list: %+v", list)
	}

	got, found, err := cli.GetOrganization(context.Background(), "evil-lab")
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if !found || got.Organization.ID != "org_1" {
		t.Fatalf("unexpected org get result: found=%v result=%+v", found, got)
	}
}

func TestUserOrgErrorResponsesCarryStatusAndCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"self_registration_disabled","message":"self-registration disabled"}`))
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	_, _, err := cli.PutCurrentUserCreate(context.Background(), UserProfileWriteRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}
	var responseErr *ErrorResponseError
	if !errors.As(err, &responseErr) {
		t.Fatalf("expected ErrorResponseError, got %T %v", err, err)
	}
	if responseErr.StatusCode != http.StatusForbidden || responseErr.Code != "self_registration_disabled" {
		t.Fatalf("unexpected response error: %+v", responseErr)
	}
	if !strings.Contains(responseErr.Error(), "self-registration disabled") {
		t.Fatalf("unexpected error message: %v", responseErr)
	}
}

func TestUserOrgOptionalLookupAndErrorBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/me":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/by-identity" && r.URL.Query().Get("subject") == "missing":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/by-identity" && r.URL.Query().Get("subject") == "forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"forbidden","message":"forbidden"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/by-identity":
			w.Header().Set("ETag", `"identity-etag"`)
			_, _ = w.Write([]byte(`{"user":{"id":"usr_3","display_name":"Lookup"},"identities":[],"memberships":[]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/users/by-identity":
			if got := r.Header.Get("If-Match"); got != `"identity-etag"` {
				t.Fatalf("If-Match = %q, want identity etag", got)
			}
			_, _ = w.Write([]byte(`{"status":"updated","user":{"id":"usr_3","display_name":"Updated"},"identities":[],"memberships":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/organizations":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"organization_slug_conflict","message":"slug already exists"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations":
			_, _ = w.Write([]byte(`null`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations/missing":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations/error":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"server_error","message":"server error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cli := New(server.URL, Options{Timeout: time.Second})
	key := IdentityKey{Provider: "oidc", Issuer: "https://issuer.example.test", Subject: "missing"}

	_, found, err := cli.GetCurrentUser(context.Background())
	if err != nil || found {
		t.Fatalf("expected current user miss, found=%v err=%v", found, err)
	}

	_, found, err = cli.GetUserByIdentity(context.Background(), key)
	if err != nil || found {
		t.Fatalf("expected identity miss, found=%v err=%v", found, err)
	}

	key.Subject = "forbidden"
	_, _, err = cli.GetUserByIdentity(context.Background(), key)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden identity lookup, got %v", err)
	}

	key.Subject = "sub-3"
	got, found, err := cli.GetUserByIdentity(context.Background(), key)
	if err != nil || !found || got.ETag != `"identity-etag"` {
		t.Fatalf("unexpected identity lookup: found=%v result=%+v err=%v", found, got, err)
	}

	updated, status, err := cli.PutUserByIdentityUpdate(context.Background(), key, `"identity-etag"`, UserProfileWriteRequest{DisplayName: "Updated"})
	if err != nil || status != http.StatusOK || updated.Status != "updated" {
		t.Fatalf("unexpected identity update: status=%d result=%+v err=%v", status, updated, err)
	}

	_, status, err = cli.CreateOrganization(context.Background(), OrganizationCreateRequest{Slug: "evil-lab"})
	if err == nil || status != http.StatusConflict || !strings.Contains(err.Error(), "slug already exists") {
		t.Fatalf("expected org conflict, status=%d err=%v", status, err)
	}

	list, err := cli.ListOrganizations(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty organization list, got %+v err=%v", list, err)
	}

	_, found, err = cli.GetOrganization(context.Background(), "missing")
	if err != nil || found {
		t.Fatalf("expected missing org, found=%v err=%v", found, err)
	}

	_, _, err = cli.GetOrganization(context.Background(), "error")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected org get server error, got %v", err)
	}

	_, _, err = cli.GetOrganization(context.Background(), " ")
	if err == nil || !strings.Contains(err.Error(), "organization reference is required") {
		t.Fatalf("expected empty org ref error, got %v", err)
	}
}
