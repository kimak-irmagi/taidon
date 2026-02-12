//go:build linux

package app

import "golang.org/x/sys/unix"

func isBtrfsPath(path string) (bool, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return false, err
	}
	return stat.Type == unix.BTRFS_SUPER_MAGIC, nil
}
