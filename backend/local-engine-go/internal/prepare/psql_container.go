package prepare

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	engineRuntime "sqlrs/engine/internal/runtime"
)

const containerScriptsRoot = "/sqlrs/scripts"

type scriptMount struct {
	HostRoot      string
	ContainerRoot string
}

func scriptMountForFiles(paths []string) (*scriptMount, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	root, err := commonDir(paths)
	if err != nil {
		return nil, err
	}
	return &scriptMount{
		HostRoot:      filepath.Clean(root),
		ContainerRoot: containerScriptsRoot,
	}, nil
}

func buildPsqlExecArgs(args []string, mount *scriptMount) ([]string, string, error) {
	rewritten, workdir, err := rewritePsqlFileArgs(args, mount)
	if err != nil {
		return nil, "", err
	}
	execArgs := []string{
		"psql",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "sqlrs",
		"-d", "postgres",
	}
	execArgs = append(execArgs, rewritten...)
	return execArgs, workdir, nil
}

func runtimeMountsFrom(mount *scriptMount) []engineRuntime.Mount {
	if mount == nil {
		return nil
	}
	return []engineRuntime.Mount{{
		HostPath:      mount.HostRoot,
		ContainerPath: mount.ContainerRoot,
		ReadOnly:      true,
	}}
}

func rewritePsqlFileArgs(args []string, mount *scriptMount) ([]string, string, error) {
	if mount == nil {
		return append([]string{}, args...), "", nil
	}
	rewritten := make([]string, 0, len(args))
	workdir := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("missing value for file flag: %s", arg)
			}
			mapped, err := mapScriptPath(args[i+1], mount)
			if err != nil {
				return nil, "", err
			}
			if workdir == "" && mapped != "-" {
				workdir = path.Dir(mapped)
			}
			rewritten = append(rewritten, arg, mapped)
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			mapped, err := mapScriptPath(value, mount)
			if err != nil {
				return nil, "", err
			}
			if workdir == "" && mapped != "-" {
				workdir = path.Dir(mapped)
			}
			rewritten = append(rewritten, "--file="+mapped)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			mapped, err := mapScriptPath(value, mount)
			if err != nil {
				return nil, "", err
			}
			if workdir == "" && mapped != "-" {
				workdir = path.Dir(mapped)
			}
			rewritten = append(rewritten, "-f"+mapped)
		default:
			rewritten = append(rewritten, arg)
		}
	}
	return rewritten, workdir, nil
}

func mapScriptPath(value string, mount *scriptMount) (string, error) {
	if value == "-" {
		return value, nil
	}
	if mount == nil {
		return "", fmt.Errorf("script mount is required for file path: %s", value)
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("file path must be absolute: %s", value)
	}
	if !isWithin(mount.HostRoot, value) {
		return "", fmt.Errorf("file path is outside script root: %s", value)
	}
	rel, err := filepath.Rel(mount.HostRoot, value)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	return path.Join(mount.ContainerRoot, rel), nil
}

func commonDir(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("paths are required")
	}
	root := filepath.Dir(filepath.Clean(paths[0]))
	for _, p := range paths[1:] {
		dir := filepath.Dir(filepath.Clean(p))
		for !isWithin(root, dir) {
			parent := filepath.Dir(root)
			if parent == root {
				return "", fmt.Errorf("paths do not share a common root")
			}
			root = parent
		}
	}
	return root, nil
}

func isWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	prefix := ".." + string(filepath.Separator)
	return !strings.HasPrefix(rel, prefix) && rel != ".."
}
