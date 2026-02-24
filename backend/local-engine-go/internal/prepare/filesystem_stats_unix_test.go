//go:build !windows

package prepare

import (
	"path/filepath"
	"testing"
)

func TestFilesystemStatsUnixReturnsCapacity(t *testing.T) {
	total, free, err := filesystemStats(t.TempDir())
	if err != nil {
		t.Fatalf("filesystemStats: %v", err)
	}
	if total <= 0 {
		t.Fatalf("expected total > 0, got %d", total)
	}
	if free < 0 {
		t.Fatalf("expected free >= 0, got %d", free)
	}
	if free > total {
		t.Fatalf("expected free <= total, got free=%d total=%d", free, total)
	}
}

func TestFilesystemStatsUnixUsesNearestExistingParent(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "nested", "cache", "states")
	total, free, err := filesystemStats(missing)
	if err != nil {
		t.Fatalf("filesystemStats: %v", err)
	}
	if total <= 0 || free < 0 {
		t.Fatalf("unexpected stats total=%d free=%d", total, free)
	}
}

func TestFilesystemStatsUnixRejectsInvalidPath(t *testing.T) {
	if _, _, err := filesystemStats("bad\x00path"); err == nil {
		t.Fatalf("expected invalid path error")
	}
}
