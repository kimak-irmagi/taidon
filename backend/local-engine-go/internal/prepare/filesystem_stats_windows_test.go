//go:build windows

package prepare

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestFilesystemStatsWindowsUsesCallerAvailableFreeBytes(t *testing.T) {
	oldFn := getDiskFreeSpaceEx
	getDiskFreeSpaceEx = func(path *uint16, freeBytesAvailable *uint64, totalBytes *uint64, totalFreeBytes *uint64) error {
		*freeBytesAvailable = 111
		*totalBytes = 999
		*totalFreeBytes = 777
		return nil
	}
	t.Cleanup(func() {
		getDiskFreeSpaceEx = oldFn
	})

	total, free, err := filesystemStats(t.TempDir())
	if err != nil {
		t.Fatalf("filesystemStats: %v", err)
	}
	if total != 999 {
		t.Fatalf("expected total=999, got %d", total)
	}
	if free != 111 {
		t.Fatalf("expected caller available free=111, got %d", free)
	}
}

func TestFilesystemStatsWindowsPropagatesGetDiskFreeSpaceExError(t *testing.T) {
	oldFn := getDiskFreeSpaceEx
	getDiskFreeSpaceEx = func(path *uint16, freeBytesAvailable *uint64, totalBytes *uint64, totalFreeBytes *uint64) error {
		return windows.ERROR_ACCESS_DENIED
	}
	t.Cleanup(func() {
		getDiskFreeSpaceEx = oldFn
	})

	if _, _, err := filesystemStats(t.TempDir()); err == nil {
		t.Fatalf("expected get disk free space error")
	}
}
