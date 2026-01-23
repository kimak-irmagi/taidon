package runkind

import "strings"

const (
	KindPsql    = "psql"
	KindPgbench = "pgbench"
)

func IsKnown(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindPsql, KindPgbench:
		return true
	default:
		return false
	}
}

func DefaultCommand(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindPsql:
		return "psql"
	case KindPgbench:
		return "pgbench"
	default:
		return ""
	}
}

func HasConnectionArgs(kind string, args []string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindPsql:
		return hasPsqlConnectionArgs(args)
	case KindPgbench:
		return hasPgbenchConnectionArgs(args)
	default:
		return false
	}
}

func hasPsqlConnectionArgs(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if isPsqlConnFlag(arg) {
			return true
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.Contains(arg, "://") {
			return true
		}
	}
	return false
}

func isPsqlConnFlag(arg string) bool {
	switch arg {
	case "-h", "-p", "-U", "-d", "--host", "--port", "--username", "--dbname", "--database":
		return true
	}
	if strings.HasPrefix(arg, "--host=") ||
		strings.HasPrefix(arg, "--port=") ||
		strings.HasPrefix(arg, "--username=") ||
		strings.HasPrefix(arg, "--dbname=") ||
		strings.HasPrefix(arg, "--database=") {
		return true
	}
	if strings.HasPrefix(arg, "-h") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-U") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-d") && len(arg) > 2 {
		return true
	}
	return false
}

func hasPgbenchConnectionArgs(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch arg {
		case "-h", "-p", "-U", "-d":
			return true
		}
		if strings.HasPrefix(arg, "-h") && len(arg) > 2 {
			return true
		}
		if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
			return true
		}
		if strings.HasPrefix(arg, "-U") && len(arg) > 2 {
			return true
		}
		if strings.HasPrefix(arg, "-d") && len(arg) > 2 {
			return true
		}
	}
	return false
}
