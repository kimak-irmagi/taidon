//go:build linux

package snapshot

import "testing"

func TestNewManagerPreferOverlayTrueUsesOverlayWhenSupported(t *testing.T) {
	prevSupported := overlaySupportedFn
	prevNew := newOverlayManagerFn
	t.Cleanup(func() {
		overlaySupportedFn = prevSupported
		newOverlayManagerFn = prevNew
	})
	overlaySupportedFn = func() bool { return true }
	newOverlayManagerFn = func() Manager {
		return overlayManager{}
	}

	mgr := NewManager(Options{PreferOverlay: true})
	if mgr.Kind() != "overlayfs" {
		t.Fatalf("expected overlayfs manager, got %s", mgr.Kind())
	}
}
