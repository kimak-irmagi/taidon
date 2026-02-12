//go:build !linux

package app

func isBtrfsPath(path string) (bool, error) {
	return false, nil
}
