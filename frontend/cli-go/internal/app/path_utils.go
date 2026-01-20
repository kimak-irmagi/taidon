package app

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
