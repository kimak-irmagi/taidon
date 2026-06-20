package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sqlrs/cli/internal/client"
)

// UserOrgOptions contains the remote API wiring for the user/org slice defined
// in docs/architecture/user-org-component-structure.md. The local engine does
// not implement these entities, so every operation validates remote mode first.
type UserOrgOptions struct {
	ProfileName string
	Mode        string
	AuthToken   string
	Endpoint    string
	Timeout     time.Duration
	Verbose     bool
}

func RunUserMe(ctx context.Context, opts UserOrgOptions) (client.UserProfileResult, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return client.UserProfileResult{}, err
	}
	result, found, err := cliClient.GetCurrentUser(ctx)
	if err != nil {
		return client.UserProfileResult{}, err
	}
	if !found {
		return client.UserProfileResult{}, fmt.Errorf("current user is not registered")
	}
	return result, nil
}

func RunUserRegister(ctx context.Context, opts UserOrgOptions, req client.UserProfileWriteRequest) (client.UserProfileResult, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return client.UserProfileResult{}, err
	}
	result, _, err := cliClient.PutCurrentUserCreate(ctx, req)
	if err == nil {
		return result, nil
	}
	var responseErr *client.ErrorResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusPreconditionFailed {
		return client.UserProfileResult{}, err
	}
	existing, found, getErr := cliClient.GetCurrentUser(ctx)
	if getErr != nil {
		return client.UserProfileResult{}, getErr
	}
	if !found {
		return client.UserProfileResult{}, err
	}
	if strings.TrimSpace(existing.Status) == "" {
		existing.Status = "existing"
	}
	return existing, nil
}

func RunUserCreate(ctx context.Context, opts UserOrgOptions, key client.IdentityKey, req client.UserProfileWriteRequest) (client.UserProfileResult, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return client.UserProfileResult{}, err
	}
	result, _, err := cliClient.PutUserByIdentityCreate(ctx, key, req)
	if err != nil {
		return client.UserProfileResult{}, err
	}
	return result, nil
}

func RunOrganizationCreate(ctx context.Context, opts UserOrgOptions, req client.OrganizationCreateRequest) (client.OrganizationCreateResponse, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return client.OrganizationCreateResponse{}, err
	}
	result, _, err := cliClient.CreateOrganization(ctx, req)
	if err != nil {
		return client.OrganizationCreateResponse{}, err
	}
	return result, nil
}

func RunOrganizationList(ctx context.Context, opts UserOrgOptions) ([]client.OrganizationMembershipView, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return nil, err
	}
	return cliClient.ListOrganizations(ctx)
}

func RunOrganizationGet(ctx context.Context, opts UserOrgOptions, orgRef string) (client.OrganizationMembershipView, error) {
	cliClient, err := remoteUserOrgClient(opts)
	if err != nil {
		return client.OrganizationMembershipView{}, err
	}
	result, found, err := cliClient.GetOrganization(ctx, orgRef)
	if err != nil {
		return client.OrganizationMembershipView{}, err
	}
	if !found {
		return client.OrganizationMembershipView{}, fmt.Errorf("organization not found: %s", strings.TrimSpace(orgRef))
	}
	return result, nil
}

func remoteUserOrgClient(opts UserOrgOptions) (*client.Client, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode != "remote" {
		return nil, fmt.Errorf("user and organization management commands require remote mode")
	}
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" || endpoint == "auto" {
		return nil, fmt.Errorf("remote mode requires explicit endpoint")
	}
	return client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: strings.TrimSpace(opts.AuthToken)}), nil
}

func PrintUserProfile(w io.Writer, result client.UserProfileResult) {
	fmt.Fprintf(w, "user: %s\n", result.User.ID)
	if result.Status != "" {
		fmt.Fprintf(w, "status: %s\n", result.Status)
	}
	if result.User.DisplayName != "" {
		fmt.Fprintf(w, "displayName: %s\n", result.User.DisplayName)
	}
	if result.User.Email != nil && *result.User.Email != "" {
		fmt.Fprintf(w, "email: %s\n", *result.User.Email)
	}
	if len(result.Identities) > 0 {
		fmt.Fprintln(w, "identities:")
		for _, identity := range result.Identities {
			fmt.Fprintf(w, "  - %s %s %s\n", identity.Provider, identity.Issuer, identity.Subject)
		}
	}
	if len(result.Memberships) > 0 {
		fmt.Fprintln(w, "memberships:")
		for _, membership := range result.Memberships {
			fmt.Fprintf(w, "  - %s %s\n", membership.Organization.Slug, membership.Membership.Role)
		}
	}
}

func PrintOrganizationMembership(w io.Writer, result client.OrganizationMembershipView) {
	fmt.Fprintf(w, "organization: %s\n", result.Organization.ID)
	fmt.Fprintf(w, "slug: %s\n", result.Organization.Slug)
	if result.Organization.DisplayName != "" {
		fmt.Fprintf(w, "name: %s\n", result.Organization.DisplayName)
	}
	if result.Membership.Role != "" {
		fmt.Fprintf(w, "role: %s\n", result.Membership.Role)
	}
}

func PrintOrganizationList(w io.Writer, rows []client.OrganizationMembershipView) {
	table := make([][]string, 0, len(rows))
	for _, row := range rows {
		table = append(table, []string{
			row.Organization.ID,
			row.Organization.Slug,
			row.Organization.DisplayName,
			row.Membership.Role,
		})
	}
	printCompactTable(w, []string{"ORG_ID", "SLUG", "NAME", "ROLE"}, table, false, compactTableColumnGap)
}
