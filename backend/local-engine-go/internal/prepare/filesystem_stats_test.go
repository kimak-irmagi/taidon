package prepare

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveFilesystemStatPathBranches(t *testing.T) {
	if _, err := resolveFilesystemStatPath(" "); err == nil {
		t.Fatalf("expected empty path validation error")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	resolved, err := resolveFilesystemStatPath(filePath)
	if err != nil {
		t.Fatalf("resolve file path: %v", err)
	}
	if resolved != dir {
		t.Fatalf("expected parent dir %q, got %q", dir, resolved)
	}

	if _, err := resolveFilesystemStatPath("bad\x00path"); err == nil {
		t.Fatalf("expected invalid path error")
	}

	missing := findMissingVolumePath()
	if missing == "" {
		t.Skip("cannot find a missing volume path on this host")
	}
	if _, err := resolveFilesystemStatPath(missing); err == nil {
		t.Fatalf("expected not-exist error for missing volume path")
	}
}

func TestClampUint64ToInt64Branches(t *testing.T) {
	if got := clampUint64ToInt64(42); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := clampUint64ToInt64(math.MaxUint64); got != math.MaxInt64 {
		t.Fatalf("expected clamp to max int64, got %d", got)
	}
}

func TestFilesystemStatsEmptyPathError(t *testing.T) {
	if _, _, err := filesystemStats(" "); err == nil {
		t.Fatalf("expected filesystemStats to reject empty path")
	}
}

func findMissingVolumePath() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	for letter := 'Z'; letter >= 'A'; letter-- {
		root := fmt.Sprintf("%c:\\", letter)
		if _, err := os.Stat(root); err != nil && errors.Is(err, os.ErrNotExist) {
			return filepath.Join(root, "sqlrs-cache", "missing")
		}
	}
	return ""
}
