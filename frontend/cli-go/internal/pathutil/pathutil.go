// Package pathutil centralizes local filesystem path normalization rules shared
// across CLI packages and tests so boundary checks stay stable across symlinked,
// short/long, and platform-specific path spellings.
package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
)

// CanonicalizeBoundaryPath resolves the existing parent path when possible so
// workspace-bound checks stay stable across symlinks and missing trailing paths.
func CanonicalizeBoundaryPath(path string) string {
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

// IsWithin reports whether target stays within base according to filepath.Rel.
func IsWithin(base string, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

// SameLocalPath reports whether two local filesystem paths point to the same
// location after separator cleanup, symlink-parent canonicalization, and
// Windows case normalization.
func SameLocalPath(left string, right string) bool {
	left = normalizeLocalPath(left)
	right = normalizeLocalPath(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func normalizeLocalPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return CanonicalizeBoundaryPath(filepath.Clean(filepath.FromSlash(trimmed)))
}
