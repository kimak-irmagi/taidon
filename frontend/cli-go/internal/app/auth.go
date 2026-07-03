package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/authsession"
	"github.com/sqlrs/cli/internal/cli"
)

type authLoginOptions = authsession.LoginOptions
type authLoginResult = authsession.LoginResult
type authStatusOptions = authsession.StatusOptions
type authStatusResult = authsession.StatusResult
type authLogoutOptions = authsession.LogoutOptions
type authLogoutResult = authsession.LogoutResult
type authResolveOptions = authsession.ResolveOptions
type authResolvedBearerToken = authsession.ResolvedBearerToken

type authManager interface {
	LoginGoogle(context.Context, authLoginOptions) (authLoginResult, error)
	Status(context.Context, authStatusOptions) (authStatusResult, error)
	Logout(context.Context, authLogoutOptions) (authLogoutResult, error)
	ResolveBearerToken(context.Context, authResolveOptions) (authResolvedBearerToken, error)
}

var authManagerFactory = func() authManager {
	return authsession.NewManager(authsession.ManagerOptions{})
}

type authInvocation struct {
	action    string
	provider  string
	loginHint string
	noBrowser bool
	noRevoke  bool
}

func parseAuthArgs(args []string) (authInvocation, bool, error) {
	var invocation authInvocation
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return invocation, false, err
	}
	if len(args) == 0 {
		return invocation, false, ExitErrorf(2, "Missing auth command")
	}
	action := strings.TrimSpace(args[0])
	if action == "--help" || action == "-h" {
		return invocation, true, nil
	}
	switch action {
	case "login":
		return parseAuthLoginArgs(args[1:])
	case "status":
		if hasHelpArg(args[1:]) {
			return invocation, true, nil
		}
		if len(args) > 1 {
			return invocation, false, ExitErrorf(2, "auth status does not accept arguments")
		}
		invocation.action = "status"
		return invocation, false, nil
	case "logout":
		return parseAuthLogoutArgs(args[1:])
	default:
		return invocation, false, ExitErrorf(2, "Unknown auth command: %s", action)
	}
}

func parseAuthLoginArgs(args []string) (authInvocation, bool, error) {
	invocation := authInvocation{action: "login"}
	if len(args) == 0 {
		return invocation, false, ExitErrorf(2, "auth login provider is required")
	}
	provider := strings.TrimSpace(args[0])
	if provider == "--help" || provider == "-h" {
		return invocation, true, nil
	}
	if provider != "google" {
		return invocation, false, ExitErrorf(2, "unsupported auth provider: %s", provider)
	}
	fs := flag.NewFlagSet("sqlrs auth login google", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	loginHint := fs.String("login-hint", "", "Google account email hint")
	noBrowser := fs.Bool("no-browser", false, "print URL instead of opening browser")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")
	if err := fs.Parse(args[1:]); err != nil {
		return invocation, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return invocation, true, nil
	}
	if fs.NArg() > 0 {
		return invocation, false, ExitErrorf(2, "auth login google does not accept positional arguments")
	}
	invocation.provider = "google"
	invocation.loginHint = strings.TrimSpace(*loginHint)
	invocation.noBrowser = *noBrowser
	return invocation, false, nil
}

func parseAuthLogoutArgs(args []string) (authInvocation, bool, error) {
	invocation := authInvocation{action: "logout", provider: "google"}
	fs := flag.NewFlagSet("sqlrs auth logout", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noRevoke := fs.Bool("no-revoke", false, "delete local credentials without revoking Google refresh token")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")
	if err := fs.Parse(args); err != nil {
		return invocation, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return invocation, true, nil
	}
	if fs.NArg() > 0 {
		return invocation, false, ExitErrorf(2, "auth logout does not accept positional arguments")
	}
	invocation.noRevoke = *noRevoke
	return invocation, false, nil
}

func runAuth(stdout, stderr io.Writer, cwd string, opts cli.GlobalOptions, args []string) error {
	invocation, showHelp, err := parseAuthArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		printAuthUsage(stdout)
		return nil
	}
	cmdCtx, err := resolveCommandContext(cwd, opts)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cmdCtx.mode) != "remote" {
		return fmt.Errorf("auth commands require remote mode")
	}
	if strings.TrimSpace(cmdCtx.profile.Endpoint) == "" || strings.TrimSpace(cmdCtx.profile.Endpoint) == "auto" {
		return fmt.Errorf("auth commands require an explicit remote endpoint")
	}

	manager := authManagerFactory()
	switch invocation.action {
	case "login":
		if !strings.EqualFold(strings.TrimSpace(cmdCtx.profile.Auth.Mode), "oidcSession") {
			return fmt.Errorf("auth login google requires profile auth.mode: oidcSession")
		}
		var authorizationURLReady func(string) error
		if invocation.noBrowser {
			urlWriter := stdout
			if cmdCtx.output == "json" {
				urlWriter = stderr
			}
			authorizationURLReady = func(authURL string) error {
				_, err := fmt.Fprintf(urlWriter, "authorizationURL: %s\n", authURL)
				return err
			}
		}
		result, err := manager.LoginGoogle(context.Background(), authLoginOptions{
			ProfileName:           cmdCtx.profileName,
			Endpoint:              cmdCtx.profile.Endpoint,
			ClientID:              cmdCtx.profile.Auth.ClientID,
			Issuer:                cmdCtx.profile.Auth.Issuer,
			LoginHint:             invocation.loginHint,
			NoBrowser:             invocation.noBrowser,
			AuthorizationURLReady: authorizationURLReady,
		})
		if err != nil {
			return err
		}
		if invocation.noBrowser && cmdCtx.output != "json" {
			result.AuthorizationURL = ""
		}
		return writeAuthLoginResult(stdout, result, cmdCtx.output)
	case "status":
		result, err := manager.Status(context.Background(), authStatusOptions{
			ProfileName: cmdCtx.profileName,
			Endpoint:    cmdCtx.profile.Endpoint,
			AuthMode:    cmdCtx.profile.Auth.Mode,
			ClientID:    cmdCtx.profile.Auth.ClientID,
			Issuer:      cmdCtx.profile.Auth.Issuer,
			TokenEnv:    cmdCtx.profile.Auth.TokenEnv,
		})
		if err != nil {
			return err
		}
		return writeAuthStatusResult(stdout, result, cmdCtx.output)
	case "logout":
		result, err := manager.Logout(context.Background(), authLogoutOptions{
			ProfileName: cmdCtx.profileName,
			Endpoint:    cmdCtx.profile.Endpoint,
			ClientID:    cmdCtx.profile.Auth.ClientID,
			Issuer:      cmdCtx.profile.Auth.Issuer,
			NoRevoke:    invocation.noRevoke,
		})
		if err != nil {
			return err
		}
		return writeAuthLogoutResult(stdout, result, cmdCtx.output)
	default:
		return fmt.Errorf("unknown auth command: %s", invocation.action)
	}
}

func resolveEffectiveAuthToken(ctx context.Context, cmdCtx commandContext) (commandContext, error) {
	if strings.TrimSpace(cmdCtx.mode) != "remote" {
		return cmdCtx, nil
	}
	if token := resolveAuthToken(cmdCtx.profile.Auth); token != "" {
		cmdCtx.authToken = token
		return cmdCtx, nil
	}
	if !strings.EqualFold(strings.TrimSpace(cmdCtx.profile.Auth.Mode), "oidcSession") {
		return cmdCtx, nil
	}
	resolved, err := authManagerFactory().ResolveBearerToken(ctx, authResolveOptions{
		ProfileName: cmdCtx.profileName,
		Endpoint:    cmdCtx.profile.Endpoint,
		AuthMode:    cmdCtx.profile.Auth.Mode,
		ClientID:    cmdCtx.profile.Auth.ClientID,
		Issuer:      cmdCtx.profile.Auth.Issuer,
		TokenEnv:    cmdCtx.profile.Auth.TokenEnv,
		StaticToken: cmdCtx.profile.Auth.Token,
	})
	if err != nil {
		return cmdCtx, err
	}
	cmdCtx.authToken = strings.TrimSpace(resolved.Token)
	return cmdCtx, nil
}

func writeAuthLoginResult(w io.Writer, result authLoginResult, output string) error {
	if output == "json" {
		return writeJSON(w, result)
	}
	if result.AuthorizationURL != "" {
		fmt.Fprintf(w, "authorizationURL: %s\n", result.AuthorizationURL)
	}
	fmt.Fprintln(w, "logged in")
	printAuthMetadata(w, result.Provider, result.Email, result.Issuer, result.Audience, result.TokenExpiry, result.Profile, result.Endpoint, "")
	return nil
}

func writeAuthStatusResult(w io.Writer, result authStatusResult, output string) error {
	if output == "json" {
		return writeJSON(w, result)
	}
	if result.LoggedIn {
		fmt.Fprintln(w, "status: logged in")
	} else {
		fmt.Fprintln(w, "status: not logged in")
	}
	printAuthMetadata(w, result.Provider, result.Email, result.Issuer, result.Audience, result.TokenExpiry, result.Profile, result.Endpoint, result.Override)
	return nil
}

func writeAuthLogoutResult(w io.Writer, result authLogoutResult, output string) error {
	if output == "json" {
		return writeJSON(w, result)
	}
	fmt.Fprintln(w, "logged out")
	if result.Provider != "" {
		fmt.Fprintf(w, "provider: %s\n", result.Provider)
	}
	if result.Profile != "" {
		fmt.Fprintf(w, "profile: %s\n", result.Profile)
	}
	if result.Endpoint != "" {
		fmt.Fprintf(w, "endpoint: %s\n", result.Endpoint)
	}
	fmt.Fprintf(w, "revoked: %t\n", result.Revoked)
	if result.RevocationFailed != "" {
		fmt.Fprintf(w, "revocationWarning: %s\n", result.RevocationFailed)
	}
	return nil
}

func printAuthMetadata(w io.Writer, provider, email, issuer, audience string, tokenExpiry interface {
	IsZero() bool
	Format(string) string
}, profile, endpoint, override string) {
	if provider != "" {
		fmt.Fprintf(w, "provider: %s\n", provider)
	}
	if email != "" {
		fmt.Fprintf(w, "email: %s\n", email)
	}
	if issuer != "" {
		fmt.Fprintf(w, "issuer: %s\n", issuer)
	}
	if audience != "" {
		fmt.Fprintf(w, "audience: %s\n", audience)
	}
	if !tokenExpiry.IsZero() {
		fmt.Fprintf(w, "tokenExpiry: %s\n", tokenExpiry.Format(timeFormatRFC3339))
	}
	if profile != "" {
		fmt.Fprintf(w, "profile: %s\n", profile)
	}
	if endpoint != "" {
		fmt.Fprintf(w, "endpoint: %s\n", endpoint)
	}
	if override != "" {
		fmt.Fprintf(w, "override: %s\n", override)
	} else {
		fmt.Fprintln(w, "override: none")
	}
}

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs auth login google [--login-hint <email>] [--no-browser]")
	fmt.Fprintln(w, "  sqlrs auth status")
	fmt.Fprintln(w, "  sqlrs auth logout [--no-revoke]")
}
