//go:build !linux

package snapshot

import "testing"

func TestBtrfsSupportedAlwaysFalse(t *testing.T) {
	if btrfsSupported("/data") {
		t.Fatalf("expected btrfs unsupported on non-linux")
	}
}
