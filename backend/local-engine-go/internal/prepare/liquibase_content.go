package prepare

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func lockLiquibaseInputs(prepared preparedRequest, changesetPath string) (*contentLock, error) {
	paths := make([]string, 0, len(prepared.liquibaseLockPaths)+1)
	paths = append(paths, prepared.liquibaseLockPaths...)
	if strings.TrimSpace(changesetPath) != "" {
		resolved := resolveLiquibaseChangesetPath(changesetPath, prepared)
		if resolved != "" {
			paths = append(paths, resolved)
		}
	}
	lockPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		path = normalizeLockPath(path)
		if path == "" || looksLikeRemoteRef(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		lockPaths = append(lockPaths, path)
	}
	return lockContentFiles(lockPaths)
}

func resolveLiquibaseChangesetPath(path string, prepared preparedRequest) string {
	path = strings.TrimSpace(path)
	if path == "" || looksLikeRemoteRef(path) {
		return ""
	}
	if looksLikeWindowsUNCPath(path) {
		return filepath.Clean(path)
	}
	if looksLikeWindowsPath(path) {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	candidates := make([]string, 0, 1+len(prepared.liquibaseSearchPaths))
	if strings.TrimSpace(prepared.liquibaseWorkDir) != "" {
		candidates = append(candidates, prepared.liquibaseWorkDir)
	}
	candidates = append(candidates, prepared.liquibaseSearchPaths...)
	for _, base := range candidates {
		if strings.TrimSpace(base) == "" {
			continue
		}
		joined := filepath.Clean(filepath.Join(base, path))
		if _, err := os.Stat(joined); err == nil {
			return joined
		}
	}
	return ""
}

func normalizeLockPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if looksLikeWindowsUNCPath(path) {
		return filepath.Clean(path)
	}
	if isWSL() && looksLikeWindowsPath(path) {
		mapped, err := wslPathConvert("-u", path)
		if err == nil {
			return mapped
		}
	}
	if looksLikeWindowsPath(path) {
		return path
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return path
}

func looksLikeWindowsUNCPath(value string) bool {
	return strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, `//`)
}

func ensureLiquibaseContentLock(prepared preparedRequest, changesetPath string) (*contentLock, *ErrorResponse) {
	lock, err := lockLiquibaseInputs(prepared, changesetPath)
	if err == nil {
		return lock, nil
	}
	return nil, errorResponse("invalid_argument", "cannot lock liquibase inputs", fmt.Sprintf("%v", err))
}
