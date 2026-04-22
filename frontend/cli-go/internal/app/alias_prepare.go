package app

import (
	"strings"

	aliaspkg "github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
)

type prepareAliasInvocation struct {
	Ref             string
	GitRef          string
	RefMode         string
	RefKeepWorktree bool
	Watch           bool
	WatchSpecified  bool
}

type planAliasInvocation struct {
	Ref             string
	GitRef          string
	RefMode         string
	RefKeepWorktree bool
}

func parsePrepareAliasArgs(args []string) (prepareAliasInvocation, bool, error) {
	opts := prepareAliasInvocation{Watch: true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h":
			return opts, true, nil
		case "--watch":
			opts.Watch = true
			opts.WatchSpecified = true
		case "--no-watch":
			opts.Watch = false
			opts.WatchSpecified = true
		case "--ref":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			opts.GitRef = value
			i++
		case "--ref-mode":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref-mode")
			}
			opts.RefMode = strings.TrimSpace(args[i+1])
			i++
		case "--ref-keep-worktree":
			opts.RefKeepWorktree = true
		case "--":
			return opts, false, ExitErrorf(2, "prepare aliases do not accept inline tool args")
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, ExitErrorf(2, "unknown prepare alias option: %s", arg)
			}
			if opts.Ref != "" {
				return opts, false, ExitErrorf(2, "prepare accepts exactly one alias ref")
			}
			opts.Ref = strings.TrimSpace(arg)
		}
	}
	if opts.Ref == "" {
		return opts, false, ExitErrorf(2, "missing prepare alias ref")
	}
	mode, err := normalizeRefMode(opts.GitRef, opts.RefMode, opts.RefKeepWorktree)
	if err != nil {
		return opts, false, err
	}
	opts.RefMode = mode
	if err := validatePrepareRefWatch(opts.GitRef, opts.WatchSpecified, opts.Watch); err != nil {
		return opts, false, err
	}
	return opts, false, nil
}

func parsePlanAliasArgs(args []string) (planAliasInvocation, bool, error) {
	opts := planAliasInvocation{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h":
			return opts, true, nil
		case "--watch", "--no-watch":
			return opts, false, ExitErrorf(2, "plan does not support --watch/--no-watch")
		case "--ref":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			value := strings.TrimSpace(args[i+1])
			if value == "" {
				return opts, false, ExitErrorf(2, "Missing value for --ref")
			}
			opts.GitRef = value
			i++
		case "--ref-mode":
			if i+1 >= len(args) {
				return opts, false, ExitErrorf(2, "Missing value for --ref-mode")
			}
			opts.RefMode = strings.TrimSpace(args[i+1])
			i++
		case "--ref-keep-worktree":
			opts.RefKeepWorktree = true
		case "--":
			return opts, false, ExitErrorf(2, "plan aliases do not accept inline tool args")
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, ExitErrorf(2, "unknown plan alias option: %s", arg)
			}
			if opts.Ref != "" {
				return opts, false, ExitErrorf(2, "plan accepts exactly one alias ref")
			}
			opts.Ref = strings.TrimSpace(arg)
		}
	}
	if opts.Ref == "" {
		return opts, false, ExitErrorf(2, "missing plan alias ref")
	}
	mode, err := normalizeRefMode(opts.GitRef, opts.RefMode, opts.RefKeepWorktree)
	if err != nil {
		return opts, false, err
	}
	opts.RefMode = mode
	return opts, false, nil
}

func resolvePrepareAliasPath(workspaceRoot string, cwd string, ref string) (string, error) {
	return resolvePrepareAliasPathWithFS(workspaceRoot, cwd, ref, inputset.OSFileSystem{})
}

func resolvePrepareAliasPathWithFS(workspaceRoot string, cwd string, ref string, fs inputset.FileSystem) (string, error) {
	target, err := aliaspkg.ResolveTargetWithFS(aliaspkg.ResolveOptions{
		WorkspaceRoot: workspaceRoot,
		CWD:           cwd,
		Ref:           ref,
		Class:         aliaspkg.ClassPrepare,
	}, fs)
	if err != nil {
		return "", wrapAliasResolveError(aliaspkg.ClassPrepare, err)
	}
	return target.Path, nil
}

func buildPrepareAliasCommandArgs(alias aliaspkg.Definition, invocation prepareAliasInvocation) []string {
	args := make([]string, 0, len(alias.Args)+3)
	args = appendRefArgs(args, invocation.GitRef, invocation.RefMode, invocation.RefKeepWorktree)
	if invocation.WatchSpecified {
		if invocation.Watch {
			args = append(args, "--watch")
		} else if strings.TrimSpace(invocation.GitRef) == "" {
			args = append(args, "--no-watch")
		}
	}
	if strings.TrimSpace(alias.Image) != "" {
		args = append(args, "--image", strings.TrimSpace(alias.Image))
	}
	args = append(args, "--")
	args = append(args, alias.Args...)
	return args
}

func buildPlanAliasCommandArgs(alias aliaspkg.Definition, invocation planAliasInvocation) []string {
	args := make([]string, 0, len(alias.Args)+3)
	args = appendRefArgs(args, invocation.GitRef, invocation.RefMode, invocation.RefKeepWorktree)
	if strings.TrimSpace(alias.Image) != "" {
		args = append(args, "--image", strings.TrimSpace(alias.Image))
	}
	args = append(args, "--")
	args = append(args, alias.Args...)
	return args
}

func appendRefArgs(dst []string, gitRef string, refMode string, refKeepWorktree bool) []string {
	gitRef = strings.TrimSpace(gitRef)
	if gitRef == "" {
		return dst
	}
	dst = append(dst, "--ref", gitRef)
	if strings.TrimSpace(refMode) != "" && refMode != "worktree" {
		dst = append(dst, "--ref-mode", refMode)
	}
	if refKeepWorktree {
		dst = append(dst, "--ref-keep-worktree")
	}
	return dst
}
