//go:build linux

package app

import "testing"

func TestResolveStoreTypeBtrfsLinux(t *testing.T) {
	t.Setenv("SQLRS_STATE_STORE", t.TempDir())
	storeType, err := resolveStoreType("btrfs", "")
	if err != nil {
		t.Fatalf("resolveStoreType: %v", err)
	}
	if storeType != "dir" && storeType != "image" {
		t.Fatalf("unexpected store type: %q", storeType)
	}
}
