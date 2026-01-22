//go:build !linux

package snapshot

import "testing"

func TestOverlayUnsupportedOnNonLinux(t *testing.T) {
	if overlaySupported() {
		t.Fatalf("expected overlay to be unsupported")
	}
	manager := newOverlayManager()
	if manager.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", manager.Kind())
	}
}
