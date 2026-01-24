package run

import "strings"

func hasPsqlConnectionArgs(args []string) bool {
	for _, arg := range args {
		value := strings.TrimSpace(arg)
		if value == "" {
			continue
		}
		if isPsqlConnFlag(value) {
			return true
		}
		if strings.HasPrefix(value, "-") {
			continue
		}
		if strings.Contains(value, "://") {
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
	for _, arg := range args {
		value := strings.TrimSpace(arg)
		if value == "" {
			continue
		}
		switch value {
		case "-h", "-p", "-U", "-d":
			return true
		}
		if strings.HasPrefix(value, "-h") && len(value) > 2 {
			return true
		}
		if strings.HasPrefix(value, "-p") && len(value) > 2 {
			return true
		}
		if strings.HasPrefix(value, "-U") && len(value) > 2 {
			return true
		}
		if strings.HasPrefix(value, "-d") && len(value) > 2 {
			return true
		}
	}
	return false
}
