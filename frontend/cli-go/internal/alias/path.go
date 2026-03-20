package alias

import (
	"os"
	"path/filepath"
	"strings"
)

func isWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	prefix := ".." + string(os.PathSeparator)
	return !strings.HasPrefix(rel, prefix) && rel != ".."
}

func canonicalizeBoundaryPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return cleaned
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}

	probe := cleaned
	suffix := make([]string, 0, 4)
	for {
		parent := filepath.Dir(probe)
		if parent == probe {
			return cleaned
		}
		suffix = append([]string{filepath.Base(probe)}, suffix...)
		probe = parent
		if resolved, err := filepath.EvalSymlinks(probe); err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Join(parts...)
		}
	}
}
