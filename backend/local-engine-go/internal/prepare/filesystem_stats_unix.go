//go:build !windows

package prepare

import "syscall"

func filesystemStats(path string) (int64, int64, error) {
	statsPath, err := resolveFilesystemStatPath(path)
	if err != nil {
		return 0, 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(statsPath, &stat); err != nil {
		return 0, 0, err
	}
	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	return clampUint64ToInt64(total), clampUint64ToInt64(free), nil
}
