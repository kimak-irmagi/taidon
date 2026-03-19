package app

import (
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli/runkind"
)

// Alias path rebasing follows the accepted contract in
// docs/adr/2026-03-19-alias-path-resolution-bases.md:
// alias refs resolve from cwd, while file-bearing args inside alias files
// resolve from the alias file directory.

func rebasePrepareAliasArgs(kind string, args []string, aliasPath string) []string {
	baseDir := filepath.Dir(aliasPath)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "psql":
		return rebasePsqlFileArgs(args, baseDir)
	case "lb":
		return rebaseLiquibasePathArgs(args, baseDir)
	default:
		return append([]string{}, args...)
	}
}

func rebaseRunAliasArgs(kind string, args []string, aliasPath string) []string {
	baseDir := filepath.Dir(aliasPath)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case runkind.KindPsql:
		return rebasePsqlFileArgs(args, baseDir)
	default:
		return append([]string{}, args...)
	}
}

func rebasePsqlFileArgs(args []string, baseDir string) []string {
	rebased := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			rebased = append(rebased, arg)
			if i+1 >= len(args) {
				continue
			}
			rebased = append(rebased, rebaseAliasFilePath(args[i+1], baseDir))
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			rebased = append(rebased, "--file="+rebaseAliasFilePath(value, baseDir))
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			rebased = append(rebased, "-f"+rebaseAliasFilePath(value, baseDir))
		default:
			rebased = append(rebased, arg)
		}
	}
	return rebased
}

func rebaseLiquibasePathArgs(args []string, baseDir string) []string {
	rebased := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file":
			rebased = append(rebased, arg)
			if i+1 >= len(args) {
				continue
			}
			value := args[i+1]
			if looksLikeLiquibaseRemoteRef(value) {
				rebased = append(rebased, value)
			} else {
				rebased = append(rebased, rebaseAliasFilePath(value, baseDir))
			}
			i++
		case arg == "--searchPath" || arg == "--search-path":
			rebased = append(rebased, arg)
			if i+1 >= len(args) {
				continue
			}
			rebased = append(rebased, rebaseLiquibaseSearchPath(args[i+1], baseDir))
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value := strings.TrimPrefix(arg, "--changelog-file=")
			if looksLikeLiquibaseRemoteRef(value) {
				rebased = append(rebased, arg)
			} else {
				rebased = append(rebased, "--changelog-file="+rebaseAliasFilePath(value, baseDir))
			}
		case strings.HasPrefix(arg, "--defaults-file="):
			value := strings.TrimPrefix(arg, "--defaults-file=")
			if looksLikeLiquibaseRemoteRef(value) {
				rebased = append(rebased, arg)
			} else {
				rebased = append(rebased, "--defaults-file="+rebaseAliasFilePath(value, baseDir))
			}
		case strings.HasPrefix(arg, "--searchPath="):
			value := strings.TrimPrefix(arg, "--searchPath=")
			rebased = append(rebased, "--searchPath="+rebaseLiquibaseSearchPath(value, baseDir))
		case strings.HasPrefix(arg, "--search-path="):
			value := strings.TrimPrefix(arg, "--search-path=")
			rebased = append(rebased, "--search-path="+rebaseLiquibaseSearchPath(value, baseDir))
		default:
			rebased = append(rebased, arg)
		}
	}
	return rebased
}

func rebaseLiquibaseSearchPath(value string, baseDir string) string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		switch {
		case item == "":
			out = append(out, item)
		case looksLikeLiquibaseRemoteRef(item):
			out = append(out, item)
		default:
			out = append(out, rebaseAliasFilePath(item, baseDir))
		}
	}
	return strings.Join(out, ",")
}

func rebaseAliasFilePath(value string, baseDir string) string {
	cleaned := strings.TrimSpace(value)
	switch {
	case cleaned == "":
		return value
	case cleaned == "-":
		return value
	case filepath.IsAbs(cleaned):
		return filepath.Clean(cleaned)
	default:
		return filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(cleaned)))
	}
}
