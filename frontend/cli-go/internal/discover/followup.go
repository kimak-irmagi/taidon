package discover

import (
	"runtime"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
)

func buildCreateCommand(ref string, class alias.Class, kind string, fileRef string) string {
	switch {
	case class == alias.ClassPrepare && kind == "psql":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "prepare:psql", "--", "-f", fileRef})
	case class == alias.ClassPrepare && kind == "lb":
		return shellJoin(liquibaseDiscoverCreateCommand(ref, fileRef))
	case class == alias.ClassRun && kind == "psql":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:psql", "--", "-f", fileRef})
	case class == alias.ClassRun && kind == "pgbench":
		return shellJoin([]string{"sqlrs", "alias", "create", ref, "run:pgbench", "--", "-f", fileRef})
	default:
		return ""
	}
}

func shellJoin(args []string) string {
	return shellJoinForGoOS(runtime.GOOS, args)
}

func shellJoinForGoOS(goos string, args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuoteForGoOS(goos, arg))
	}
	return strings.Join(parts, " ")
}

func shellQuoteForGoOS(goos string, value string) string {
	if value == "" {
		return "''"
	}
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		if isPowerShellBareWord(value) {
			return value
		}
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	default:
		if !strings.ContainsAny(value, " \t\n\r'\"$&|<>;()[]{}*?!`\\") {
			return value
		}
		return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
	}
}

func isPowerShellBareWord(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '/' || r == ':' || r == '-':
		default:
			return false
		}
	}
	return true
}
