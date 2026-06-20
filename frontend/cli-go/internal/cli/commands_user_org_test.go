package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/client"
)

func TestRunUserRegisterCreatedAndPrintHuman(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/users/me" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("If-None-Match") != "*" {
			t.Fatalf("expected create-only conditional request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created","user":{"id":"usr_1","display_name":"Evil Guest","email":"evil@example.test"},"identities":[{"provider":"oidc","issuer":"https://issuer.example.test","subject":"sub-1"}],"memberships":[]}`))
	}))
	defer server.Close()

	email := "evil@example.test"
	result, err := RunUserRegister(context.Background(), UserOrgOptions{
		Mode:      "remote",
		Endpoint:  server.URL,
		AuthToken: "secret",
		Timeout:   time.Second,
	}, client.UserProfileWriteRequest{DisplayName: "Evil Guest", Email: &email})
	if err != nil {
		t.Fatalf("RunUserRegister: %v", err)
	}
	if result.Status != "created" || result.User.ID != "usr_1" {
		t.Fatalf("unexpected register result: %+v", result)
	}

	var out bytes.Buffer
	PrintUserProfile(&out, result)
	if got := out.String(); !strings.Contains(got, "user: usr_1") || !strings.Contains(got, "status: created") || !strings.Contains(got, "oidc https://issuer.example.test sub-1") {
		t.Fatalf("unexpected user output: %q", got)
	}
}

func TestRunUserRegisterPreconditionFetchesCurrentUser(t *testing.T) {
	putCalls := 0
	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/users/me":
			putCalls++
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"code":"identity_already_linked","message":"identity already linked"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/me":
			getCalls++
			_, _ = w.Write([]byte(`{"status":"existing","user":{"id":"usr_1","display_name":"Evil Guest"},"identities":[],"memberships":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunUserRegister(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second}, client.UserProfileWriteRequest{})
	if err != nil {
		t.Fatalf("RunUserRegister: %v", err)
	}
	if result.Status != "existing" || result.User.ID != "usr_1" {
		t.Fatalf("unexpected existing user result: %+v", result)
	}
	if putCalls != 1 || getCalls != 1 {
		t.Fatalf("expected PUT then GET, got put=%d get=%d", putCalls, getCalls)
	}
}

func TestRunUserCreateDuplicateReturnsPreconditionError(t *testing.T) {
	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getCalls++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"code":"identity_already_linked","message":"identity already linked"}`))
	}))
	defer server.Close()

	_, err := RunUserCreate(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second}, client.IdentityKey{
		Provider: "oidc",
		Issuer:   "https://issuer.example.test",
		Subject:  "sub-1",
	}, client.UserProfileWriteRequest{DisplayName: "New User"})
	if err == nil {
		t.Fatalf("expected duplicate identity error")
	}
	var responseErr *client.ErrorResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 response error, got %T %v", err, err)
	}
	if getCalls != 0 {
		t.Fatalf("user create must not follow duplicate with GET, got %d GET calls", getCalls)
	}
}

func TestRunUserMeSuccessNotRegisteredAndServerError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/users/me" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"existing","user":{"id":"usr_1","display_name":"Evil Guest"},"identities":[],"memberships":[]}`))
		}))
		defer server.Close()

		result, err := RunUserMe(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second})
		if err != nil {
			t.Fatalf("RunUserMe: %v", err)
		}
		if result.User.ID != "usr_1" {
			t.Fatalf("unexpected user me result: %+v", result)
		}
	})

	t.Run("not registered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := RunUserMe(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second})
		if err == nil || !strings.Contains(err.Error(), "not registered") {
			t.Fatalf("expected not registered error, got %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"forbidden","message":"forbidden"}`))
		}))
		defer server.Close()

		_, err := RunUserMe(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second})
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("expected server error, got %v", err)
		}
	})
}

func TestRunUserRegisterPreconditionUsesDefaultExistingStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"code":"identity_already_linked","message":"identity already linked"}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"user":{"id":"usr_1","display_name":"Evil Guest"},"identities":[],"memberships":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunUserRegister(context.Background(), UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second}, client.UserProfileWriteRequest{})
	if err != nil {
		t.Fatalf("RunUserRegister: %v", err)
	}
	if result.Status != "existing" {
		t.Fatalf("expected default existing status, got %+v", result)
	}
}

func TestRunOrganizationCreateListGetAndPrint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/organizations":
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

	opts := UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second}
	created, err := RunOrganizationCreate(context.Background(), opts, client.OrganizationCreateRequest{Slug: "evil-lab", DisplayName: "Evil Lab"})
	if err != nil {
		t.Fatalf("RunOrganizationCreate: %v", err)
	}
	if created.Membership.Role != "admin" {
		t.Fatalf("unexpected create result: %+v", created)
	}

	list, err := RunOrganizationList(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunOrganizationList: %v", err)
	}
	if len(list) != 1 || list[0].Organization.Slug != "evil-lab" {
		t.Fatalf("unexpected org list: %+v", list)
	}

	got, err := RunOrganizationGet(context.Background(), opts, "evil-lab")
	if err != nil {
		t.Fatalf("RunOrganizationGet: %v", err)
	}
	if got.Organization.ID != "org_1" {
		t.Fatalf("unexpected org get result: %+v", got)
	}

	var out bytes.Buffer
	PrintOrganizationMembership(&out, got)
	PrintOrganizationList(&out, list)
	human := out.String()
	if !strings.Contains(human, "organization: org_1") || !strings.Contains(human, "slug: evil-lab") || !strings.Contains(human, "admin") {
		t.Fatalf("unexpected organization output: %q", human)
	}
}

func TestRunOrganizationGetNotFoundAndCreateConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/organizations":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"organization_slug_conflict","message":"slug already exists"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/organizations/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := UserOrgOptions{Mode: "remote", Endpoint: server.URL, Timeout: time.Second}
	_, err := RunOrganizationGet(context.Background(), opts, "missing")
	if err == nil || !strings.Contains(err.Error(), "organization not found") {
		t.Fatalf("expected organization not found, got %v", err)
	}

	_, err = RunOrganizationCreate(context.Background(), opts, client.OrganizationCreateRequest{Slug: "evil-lab"})
	if err == nil || !strings.Contains(err.Error(), "slug already exists") {
		t.Fatalf("expected slug conflict, got %v", err)
	}
}

func TestPrintUserProfileIncludesMemberships(t *testing.T) {
	var out bytes.Buffer
	PrintUserProfile(&out, client.UserProfileResult{
		User: client.UserProfile{ID: "usr_1"},
		Memberships: []client.OrganizationMembershipView{
			{
				Organization: client.Organization{Slug: "evil-lab"},
				Membership:   client.OrganizationMembership{Role: "admin"},
			},
		},
	})
	if got := out.String(); !strings.Contains(got, "memberships:") || !strings.Contains(got, "evil-lab admin") {
		t.Fatalf("unexpected membership output: %q", got)
	}
}

func TestUserOrgRemoteModeValidation(t *testing.T) {
	_, err := RunUserMe(context.Background(), UserOrgOptions{Mode: "local", Endpoint: "auto"})
	if err == nil || !strings.Contains(err.Error(), "require remote mode") {
		t.Fatalf("expected local mode rejection, got %v", err)
	}

	_, err = RunOrganizationList(context.Background(), UserOrgOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected remote endpoint error, got %v", err)
	}
}
