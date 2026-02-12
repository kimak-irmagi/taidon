package prepare

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type PathMapper interface {
	MapPath(path string) (string, error)
}

var (
	getEnv = os.Getenv
	setEnv = os.Setenv
)

type wslPathMapper struct{}

func (w wslPathMapper) MapPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return path, nil
	}
	if looksLikeWindowsPath(trimmed) {
		return trimmed, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		return trimmed, nil
	}
	mapped, err := wslPathConvert("-m", trimmed)
	if err == nil {
		return normalizeWindowsPath(mapped), nil
	}
	return wslPathConvert("-w", trimmed)
}

func isWSL() bool {
	return strings.TrimSpace(getEnv("WSL_INTEROP")) != "" || strings.TrimSpace(getEnv("WSL_DISTRO_NAME")) != ""
}

func defaultLiquibasePathMapper() PathMapper {
	if isWSL() {
		return wslPathMapper{}
	}
	return nil
}

func normalizeLiquibaseExecPath(execPath string, windowsMode bool) (string, error) {
	execPath = sanitizeLiquibaseExecPath(execPath)
	if execPath == "" {
		return "", nil
	}
	if windowsMode {
		return execPath, nil
	}
	if !looksLikeWindowsPath(execPath) {
		return execPath, nil
	}
	if !isWSL() {
		return execPath, nil
	}
	return wslPathConvert("-u", execPath)
}

func sanitizeLiquibaseExecPath(execPath string) string {
	execPath = strings.TrimSpace(execPath)
	if execPath == "" {
		return ""
	}
	if strings.Contains(execPath, `\"`) {
		execPath = strings.ReplaceAll(execPath, `\"`, `"`)
	}
	if len(execPath) >= 2 {
		if (execPath[0] == '"' && execPath[len(execPath)-1] == '"') ||
			(execPath[0] == '\'' && execPath[len(execPath)-1] == '\'') {
			execPath = execPath[1 : len(execPath)-1]
		}
	}
	return strings.TrimSpace(execPath)
}

func mapLiquibaseArgs(args []string, mapper PathMapper) ([]string, error) {
	if mapper == nil {
		return append([]string{}, args...), nil
	}
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file" || arg == "--searchPath":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for %s", arg)
			}
			value := args[i+1]
			rewritten, err := mapLiquibasePathValue(arg, value, mapper)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, arg, rewritten)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value := strings.TrimPrefix(arg, "--changelog-file=")
			rewritten, err := mapLiquibasePathValue("--changelog-file", value, mapper)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--changelog-file="+rewritten)
		case strings.HasPrefix(arg, "--defaults-file="):
			value := strings.TrimPrefix(arg, "--defaults-file=")
			rewritten, err := mapLiquibasePathValue("--defaults-file", value, mapper)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--defaults-file="+rewritten)
		case strings.HasPrefix(arg, "--searchPath="):
			value := strings.TrimPrefix(arg, "--searchPath=")
			rewritten, err := mapLiquibasePathValue("--searchPath", value, mapper)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--searchPath="+rewritten)
		default:
			normalized = append(normalized, arg)
		}
	}
	return normalized, nil
}

func mapLiquibaseEnv(env map[string]string, windowsMode bool) (map[string]string, error) {
	if len(env) == 0 {
		return nil, nil
	}
	javaHome := strings.TrimSpace(env["JAVA_HOME"])
	if javaHome == "" {
		return env, nil
	}
	if !isWSL() {
		return env, nil
	}
	if windowsMode {
		return env, nil
	}
	if !looksLikeWindowsPath(javaHome) {
		return env, nil
	}
	mapped, err := wslPathConvert("-u", javaHome)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = value
	}
	out["JAVA_HOME"] = mapped
	return out, nil
}

func mapLiquibasePathValue(flag string, value string, mapper PathMapper) (string, error) {
	if flag == "--searchPath" {
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("searchPath is empty")
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				return "", fmt.Errorf("searchPath is empty")
			}
			if looksLikeRemoteRef(item) {
				out = append(out, item)
				continue
			}
			mapped, err := mapper.MapPath(item)
			if err != nil {
				return "", err
			}
			out = append(out, mapped)
		}
		return strings.Join(out, ","), nil
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("path is empty")
	}
	if looksLikeRemoteRef(value) {
		return value, nil
	}
	return mapper.MapPath(value)
}

func looksLikeWindowsPath(value string) bool {
	if len(value) < 2 {
		return false
	}
	if value[1] != ':' {
		return false
	}
	letter := value[0]
	if (letter < 'A' || letter > 'Z') && (letter < 'a' || letter > 'z') {
		return false
	}
	return true
}

func wslPathConvert(flag string, path string) (string, error) {
	cmd := execCommand(context.Background(), "wslpath", flag, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wslpath %s failed: %w", flag, err)
	}
	converted := strings.TrimSpace(string(output))
	if converted == "" {
		return "", fmt.Errorf("wslpath %s returned empty output", flag)
	}
	return converted, nil
}

func normalizeWindowsPath(path string) string {
	if strings.Contains(path, "/") && !strings.Contains(path, `\`) {
		return strings.ReplaceAll(path, "/", `\`)
	}
	return path
}
