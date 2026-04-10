package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const prepareAliasSuffix = ".prep.s9s.yaml"

type prepareAlias struct {
	Kind  string   `yaml:"kind"`
	Image string   `yaml:"image"`
	Args  []string `yaml:"args"`
}

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
	base := strings.TrimSpace(cwd)
	if base == "" {
		base = strings.TrimSpace(workspaceRoot)
	}
	if base == "" {
		return "", ExitErrorf(2, "workspace root is required to resolve prepare aliases")
	}
	base = filepath.Clean(base)
	boundary := strings.TrimSpace(workspaceRoot)
	if boundary == "" {
		boundary = base
	}
	boundary = filepath.Clean(boundary)

	cleanedRef := strings.TrimSpace(ref)
	if cleanedRef == "" {
		return "", ExitErrorf(2, "missing prepare alias ref")
	}

	exact := strings.HasSuffix(cleanedRef, ".")
	if exact {
		cleanedRef = strings.TrimSuffix(cleanedRef, ".")
		if strings.TrimSpace(cleanedRef) == "" {
			return "", ExitErrorf(2, "prepare alias ref is empty")
		}
	}

	relativePath := filepath.FromSlash(cleanedRef)
	if !exact {
		relativePath += prepareAliasSuffix
	}

	resolved, _, err := normalizeFilePath(relativePath, boundary, base, nil)
	if err != nil {
		return "", err
	}
	if !prepareAliasFileExists(resolved) {
		return "", ExitErrorf(2, "prepare alias file not found: %s", resolved)
	}
	return resolved, nil
}

func loadPrepareAlias(path string) (prepareAlias, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return prepareAlias{}, err
	}
	var alias prepareAlias
	if err := yaml.Unmarshal(data, &alias); err != nil {
		return prepareAlias{}, fmt.Errorf("read prepare alias: %w", err)
	}
	alias.Kind = strings.ToLower(strings.TrimSpace(alias.Kind))
	switch alias.Kind {
	case "":
		return prepareAlias{}, ExitErrorf(2, "prepare alias kind is required")
	case "psql", "lb":
	default:
		return prepareAlias{}, ExitErrorf(2, "unknown prepare alias kind: %s", alias.Kind)
	}
	if len(alias.Args) == 0 {
		return prepareAlias{}, ExitErrorf(2, "prepare alias args are required")
	}
	return alias, nil
}

func buildPrepareAliasCommandArgs(alias prepareAlias, invocation prepareAliasInvocation) []string {
	args := make([]string, 0, len(alias.Args)+3)
	args = appendRefArgs(args, invocation.GitRef, invocation.RefMode, invocation.RefKeepWorktree)
	if invocation.WatchSpecified {
		if invocation.Watch {
			args = append(args, "--watch")
		} else {
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

func buildPlanAliasCommandArgs(alias prepareAlias, invocation planAliasInvocation) []string {
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

func prepareAliasFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
