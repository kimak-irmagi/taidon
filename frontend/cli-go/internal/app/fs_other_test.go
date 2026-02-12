//go:build !linux

package app

import "testing"

func TestIsBtrfsPathAlwaysFalse(t *testing.T) {
	ok, err := isBtrfsPath("C:\\")
	if err != nil {
		t.Fatalf("isBtrfsPath: %v", err)
	}
	if ok {
		t.Fatalf("expected btrfs=false on non-linux")
	}
}
