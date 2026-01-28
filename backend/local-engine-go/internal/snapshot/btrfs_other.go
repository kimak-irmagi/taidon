//go:build !linux

package snapshot

func btrfsSupported(path string) bool {
	return false
}

func newBtrfsManager() Manager {
	return CopyManager{}
}
