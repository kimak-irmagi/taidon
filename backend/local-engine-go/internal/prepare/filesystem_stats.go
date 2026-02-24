package prepare

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveFilesystemStatPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("state store root is required")
	}
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if info.IsDir() {
				return current, nil
			}
			return filepath.Dir(current), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		current = parent
	}
}

func clampUint64ToInt64(value uint64) int64 {
	const maxInt64Uint = uint64(^uint64(0) >> 1)
	if value > maxInt64Uint {
		return int64(maxInt64Uint)
	}
	return int64(value)
}
