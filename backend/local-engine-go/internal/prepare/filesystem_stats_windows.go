//go:build windows

package prepare

import "golang.org/x/sys/windows"

var getDiskFreeSpaceEx = windows.GetDiskFreeSpaceEx

func filesystemStats(path string) (int64, int64, error) {
	statsPath, err := resolveFilesystemStatPath(path)
	if err != nil {
		return 0, 0, err
	}
	ptr, err := windows.UTF16PtrFromString(statsPath)
	if err != nil {
		return 0, 0, err
	}
	var freeBytesAvailable uint64
	var totalBytes uint64
	var totalFreeBytes uint64
	if err := getDiskFreeSpaceEx(ptr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, 0, err
	}
	return clampUint64ToInt64(totalBytes), clampUint64ToInt64(freeBytesAvailable), nil
}
