package snapshot

import "testing"

func TestNewManagerPreferOverlayFalseReturnsCopyManager(t *testing.T) {
	mgr := NewManager(Options{PreferOverlay: false})
	if mgr.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", mgr.Kind())
	}
}
