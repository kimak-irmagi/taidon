//go:build !linux

package snapshot

import "testing"

func TestBtrfsSupportedAlwaysFalse(t *testing.T) {
	if btrfsSupported("/data") {
		t.Fatalf("expected btrfs unsupported on non-linux")
	}
}

func TestNewBtrfsManagerReturnsCopy(t *testing.T) {
	mgr := newBtrfsManager()
	if mgr.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", mgr.Kind())
	}
}
