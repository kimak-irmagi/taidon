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
	Ref            string
	Watch          bool
	WatchSpecified bool
}

func parsePrepareAliasArgs(args []string) (prepareAliasInvocation, bool, error) {
	opts := prepareAliasInvocation{Watch: true}
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return opts, true, nil
		case "--watch":
			opts.Watch = true
			opts.WatchSpecified = true
		case "--no-watch":
			opts.Watch = false
			opts.WatchSpecified = true
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
	return opts, false, nil
}

func parsePlanAliasArgs(args []string) (string, bool, error) {
	ref := ""
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return "", true, nil
		case "--watch", "--no-watch":
			return "", false, ExitErrorf(2, "plan does not support --watch/--no-watch")
		case "--":
			return "", false, ExitErrorf(2, "plan aliases do not accept inline tool args")
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, ExitErrorf(2, "unknown plan alias option: %s", arg)
			}
			if ref != "" {
				return "", false, ExitErrorf(2, "plan accepts exactly one alias ref")
			}
			ref = strings.TrimSpace(arg)
		}
	}
	if ref == "" {
		return "", false, ExitErrorf(2, "missing plan alias ref")
	}
	return ref, false, nil
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

func buildPlanAliasCommandArgs(alias prepareAlias) []string {
	args := make([]string, 0, len(alias.Args)+3)
	if strings.TrimSpace(alias.Image) != "" {
		args = append(args, "--image", strings.TrimSpace(alias.Image))
	}
	args = append(args, "--")
	args = append(args, alias.Args...)
	return args
}

func prepareAliasFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
