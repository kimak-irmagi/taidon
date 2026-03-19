package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli/runkind"
	"gopkg.in/yaml.v3"
)

const runAliasSuffix = ".run.s9s.yaml"

type runAlias struct {
	Kind  string   `yaml:"kind"`
	Image string   `yaml:"image"`
	Args  []string `yaml:"args"`
}

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
	base := strings.TrimSpace(cwd)
	if base == "" {
		base = strings.TrimSpace(workspaceRoot)
	}
	if base == "" {
		return "", ExitErrorf(2, "workspace root is required to resolve run aliases")
	}
	base = filepath.Clean(base)
	boundary := strings.TrimSpace(workspaceRoot)
	if boundary == "" {
		boundary = base
	}
	boundary = filepath.Clean(boundary)

	cleanedRef := strings.TrimSpace(ref)
	if cleanedRef == "" {
		return "", ExitErrorf(2, "missing run alias ref")
	}

	exact := strings.HasSuffix(cleanedRef, ".")
	if exact {
		cleanedRef = strings.TrimSuffix(cleanedRef, ".")
		if strings.TrimSpace(cleanedRef) == "" {
			return "", ExitErrorf(2, "run alias ref is empty")
		}
	}

	relativePath := filepath.FromSlash(cleanedRef)
	if !exact {
		relativePath += runAliasSuffix
	}

	resolved, _, err := normalizeFilePath(relativePath, boundary, base, nil)
	if err != nil {
		return "", err
	}
	if !runAliasFileExists(resolved) {
		return "", ExitErrorf(2, "run alias file not found: %s", resolved)
	}
	return resolved, nil
}

func loadRunAlias(path string) (runAlias, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runAlias{}, err
	}
	var alias runAlias
	if err := yaml.Unmarshal(data, &alias); err != nil {
		return runAlias{}, fmt.Errorf("read run alias: %w", err)
	}
	alias.Kind = strings.ToLower(strings.TrimSpace(alias.Kind))
	switch alias.Kind {
	case "":
		return runAlias{}, ExitErrorf(2, "run alias kind is required")
	default:
		if !runkind.IsKnown(alias.Kind) {
			return runAlias{}, ExitErrorf(2, "unknown run alias kind: %s", alias.Kind)
		}
	}
	if strings.TrimSpace(alias.Image) != "" {
		return runAlias{}, ExitErrorf(2, "run alias does not support image")
	}
	if len(alias.Args) == 0 {
		return runAlias{}, ExitErrorf(2, "run alias args are required")
	}
	return alias, nil
}

func buildRunAliasCommandArgs(alias runAlias, invocation runAliasInvocation) []string {
	args := make([]string, 0, len(alias.Args)+2)
	if strings.TrimSpace(invocation.InstanceRef) != "" {
		args = append(args, "--instance", strings.TrimSpace(invocation.InstanceRef))
	}
	args = append(args, alias.Args...)
	return args
}

func runAliasFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
