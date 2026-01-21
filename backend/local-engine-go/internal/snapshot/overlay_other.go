//go:build !linux

package snapshot

func overlaySupported() bool {
	return false
}

func newOverlayManager() Manager {
	return CopyManager{}
}
