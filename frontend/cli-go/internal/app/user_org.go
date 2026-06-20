package app

import (
	"context"
	"flag"
	"io"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
)

type userCommand struct {
	action   string
	identity client.IdentityKey
	profile  client.UserProfileWriteRequest
}

type orgCommand struct {
	action      string
	slug        string
	ref         string
	displayName string
}

func parseUserArgs(args []string) (userCommand, bool, error) {
	var cmd userCommand
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return cmd, false, err
	}
	if len(args) == 0 {
		return cmd, false, ExitErrorf(2, "Missing user command")
	}
	action := strings.TrimSpace(args[0])
	if action == "--help" || action == "-h" {
		return cmd, true, nil
	}

	switch action {
	case "me":
		if hasHelpArg(args[1:]) {
			return cmd, true, nil
		}
		if len(args) > 1 {
			return cmd, false, ExitErrorf(2, "user me does not accept arguments")
		}
		cmd.action = "me"
	case "register":
		parsed, showHelp, err := parseUserRegisterArgs(args[1:])
		if err != nil || showHelp {
			return parsed, showHelp, err
		}
		cmd = parsed
	case "create":
		parsed, showHelp, err := parseUserCreateArgs(args[1:])
		if err != nil || showHelp {
			return parsed, showHelp, err
		}
		cmd = parsed
	default:
		return cmd, false, ExitErrorf(2, "Unknown user command: %s", action)
	}
	return cmd, false, nil
}

func parseUserRegisterArgs(args []string) (userCommand, bool, error) {
	cmd := userCommand{action: "register"}
	fs := flag.NewFlagSet("sqlrs user register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	displayName := fs.String("display-name", "", "display name")
	email := fs.String("email", "", "email")
	identityProvider := fs.String("identity-provider", "", "external identity provider")
	identityIssuer := fs.String("identity-issuer", "", "external identity issuer")
	identitySubject := fs.String("identity-subject", "", "external identity subject")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return cmd, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return cmd, true, nil
	}
	if fs.NArg() > 0 {
		return cmd, false, ExitErrorf(2, "user register does not accept positional arguments")
	}
	if strings.TrimSpace(*identityProvider) != "" || strings.TrimSpace(*identityIssuer) != "" || strings.TrimSpace(*identitySubject) != "" {
		return cmd, false, ExitErrorf(2, "user register identity is derived from the authenticated token; explicit identity flags are not accepted")
	}
	cmd.profile = userProfileWriteRequest(*displayName, *email)
	return cmd, false, nil
}

func parseUserCreateArgs(args []string) (userCommand, bool, error) {
	cmd := userCommand{action: "create"}
	fs := flag.NewFlagSet("sqlrs user create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	displayName := fs.String("display-name", "", "display name")
	email := fs.String("email", "", "email")
	identityProvider := fs.String("identity-provider", "oidc", "external identity provider")
	identityIssuer := fs.String("identity-issuer", "", "external identity issuer")
	identitySubject := fs.String("identity-subject", "", "external identity subject")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return cmd, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return cmd, true, nil
	}
	if fs.NArg() > 0 {
		return cmd, false, ExitErrorf(2, "user create does not accept positional arguments")
	}
	provider := strings.TrimSpace(*identityProvider)
	if provider == "" {
		provider = "oidc"
	}
	issuer := strings.TrimSpace(*identityIssuer)
	subject := strings.TrimSpace(*identitySubject)
	if issuer == "" {
		return cmd, false, ExitErrorf(2, "identity issuer is required")
	}
	if subject == "" {
		return cmd, false, ExitErrorf(2, "identity subject is required")
	}
	cmd.identity = client.IdentityKey{Provider: provider, Issuer: issuer, Subject: subject}
	cmd.profile = userProfileWriteRequest(*displayName, *email)
	return cmd, false, nil
}

func userProfileWriteRequest(displayName, email string) client.UserProfileWriteRequest {
	req := client.UserProfileWriteRequest{DisplayName: strings.TrimSpace(displayName)}
	if strings.TrimSpace(email) != "" {
		value := strings.TrimSpace(email)
		req.Email = &value
	}
	return req
}

func parseOrgArgs(args []string) (orgCommand, bool, error) {
	var cmd orgCommand
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return cmd, false, err
	}
	if len(args) == 0 {
		return cmd, false, ExitErrorf(2, "Missing organization command")
	}
	action := strings.TrimSpace(args[0])
	if action == "--help" || action == "-h" {
		return cmd, true, nil
	}

	switch action {
	case "create":
		return parseOrgCreateArgs(args[1:])
	case "ls":
		if hasHelpArg(args[1:]) {
			return cmd, true, nil
		}
		if len(args) > 1 {
			return cmd, false, ExitErrorf(2, "org ls does not accept arguments")
		}
		cmd.action = "ls"
		return cmd, false, nil
	case "get":
		if hasHelpArg(args[1:]) {
			return cmd, true, nil
		}
		if len(args) < 2 {
			return cmd, false, ExitErrorf(2, "organization reference is required")
		}
		if len(args) > 2 {
			return cmd, false, ExitErrorf(2, "too many org get arguments")
		}
		cmd.action = "get"
		cmd.ref = strings.TrimSpace(args[1])
		if cmd.ref == "" {
			return cmd, false, ExitErrorf(2, "organization reference is required")
		}
		return cmd, false, nil
	default:
		return cmd, false, ExitErrorf(2, "Unknown organization command: %s", action)
	}
}

func parseOrgCreateArgs(args []string) (orgCommand, bool, error) {
	cmd := orgCommand{action: "create"}
	fs := flag.NewFlagSet("sqlrs org create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	displayName := fs.String("name", "", "organization display name")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return cmd, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}
	if *help || *helpShort {
		return cmd, true, nil
	}
	if fs.NArg() == 0 {
		return cmd, false, ExitErrorf(2, "organization slug is required")
	}
	if fs.NArg() > 1 {
		return cmd, false, ExitErrorf(2, "too many org create arguments")
	}
	cmd.slug = strings.TrimSpace(fs.Arg(0))
	if cmd.slug == "" {
		return cmd, false, ExitErrorf(2, "organization slug is required")
	}
	cmd.displayName = strings.TrimSpace(*displayName)
	return cmd, false, nil
}

func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == "--help" || strings.TrimSpace(arg) == "-h" {
			return true
		}
	}
	return false
}

func runUser(stdout io.Writer, cmdCtx commandContext, args []string, output string) error {
	parsed, showHelp, err := parseUserArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintUserUsage(stdout)
		return nil
	}

	opts := cmdCtx.userOrgOptions()
	switch parsed.action {
	case "me":
		result, err := cli.RunUserMe(context.Background(), opts)
		if err != nil {
			return err
		}
		return writeUserProfile(stdout, result, output)
	case "register":
		result, err := cli.RunUserRegister(context.Background(), opts, parsed.profile)
		if err != nil {
			return err
		}
		return writeUserProfile(stdout, result, output)
	}

	result, err := cli.RunUserCreate(context.Background(), opts, parsed.identity, parsed.profile)
	if err != nil {
		return err
	}
	return writeUserProfile(stdout, result, output)
}

func runOrg(stdout io.Writer, cmdCtx commandContext, args []string, output string) error {
	parsed, showHelp, err := parseOrgArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintOrgUsage(stdout)
		return nil
	}

	opts := cmdCtx.userOrgOptions()
	switch parsed.action {
	case "create":
		result, err := cli.RunOrganizationCreate(context.Background(), opts, client.OrganizationCreateRequest{
			Slug:        parsed.slug,
			DisplayName: parsed.displayName,
		})
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, result)
		}
		cli.PrintOrganizationMembership(stdout, client.OrganizationMembershipView{
			Organization: result.Organization,
			Membership:   result.Membership,
		})
		return nil
	case "ls":
		result, err := cli.RunOrganizationList(context.Background(), opts)
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, result)
		}
		cli.PrintOrganizationList(stdout, result)
		return nil
	}

	result, err := cli.RunOrganizationGet(context.Background(), opts, parsed.ref)
	if err != nil {
		return err
	}
	if output == "json" {
		return writeJSON(stdout, result)
	}
	cli.PrintOrganizationMembership(stdout, result)
	return nil
}

func writeUserProfile(stdout io.Writer, result client.UserProfileResult, output string) error {
	if output == "json" {
		return writeJSON(stdout, result)
	}
	cli.PrintUserProfile(stdout, result)
	return nil
}
