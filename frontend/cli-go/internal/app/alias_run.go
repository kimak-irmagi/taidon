package app

import (
	"strings"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
)

type runAliasInvocation struct {
	Ref         string
	InstanceRef string
}

func parseRunAliasArgs(args []string, requireInstance bool) (runAliasInvocation, bool, error) {
	var opts runAliasInvocation
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return opts, false, err
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return opts, true, nil
		case arg == "--":
			return opts, false, ExitErrorf(2, "run aliases do not accept inline tool args")
		case arg == "--instance":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			opts.InstanceRef = value
			i++
		case strings.HasPrefix(arg, "--instance="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--instance="))
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --instance")
			}
			opts.InstanceRef = value
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, ExitErrorf(2, "unknown run alias option: %s", arg)
			}
			if opts.Ref != "" {
				return opts, false, ExitErrorf(2, "run accepts exactly one alias ref")
			}
			opts.Ref = strings.TrimSpace(arg)
		}
	}
	if opts.Ref == "" {
		return opts, false, ExitErrorf(2, "missing run alias ref")
	}
	if requireInstance && strings.TrimSpace(opts.InstanceRef) == "" {
		return opts, false, ExitErrorf(2, "run alias requires --instance when used without prepare")
	}
	return opts, false, nil
}

func resolveRunAliasPath(workspaceRoot string, cwd string, ref string) (string, error) {
	target, err := aliaspkg.ResolveTarget(aliaspkg.ResolveOptions{
		WorkspaceRoot: workspaceRoot,
		CWD:           cwd,
		Ref:           ref,
		Class:         aliaspkg.ClassRun,
	})
	if err != nil {
		return "", wrapAliasResolveError(aliaspkg.ClassRun, err)
	}
	return target.Path, nil
}

func resolveRunAliasDefinition(workspaceRoot string, cwd string, ref string) (aliaspkg.Definition, string, error) {
	aliasPath, err := resolveRunAliasPath(workspaceRoot, cwd, ref)
	if err != nil {
		return aliaspkg.Definition{}, "", err
	}
	def, err := aliaspkg.LoadTarget(aliaspkg.Target{Class: aliaspkg.ClassRun, Path: aliasPath})
	if err != nil {
		return aliaspkg.Definition{}, "", wrapAliasLoadError(err)
	}
	return def, aliasPath, nil
}

func buildRunAliasCommandArgs(alias aliaspkg.Definition, invocation runAliasInvocation) []string {
	args := make([]string, 0, len(alias.Args)+2)
	if strings.TrimSpace(invocation.InstanceRef) != "" {
		args = append(args, "--instance", strings.TrimSpace(invocation.InstanceRef))
	}
	args = append(args, alias.Args...)
	return args
}
