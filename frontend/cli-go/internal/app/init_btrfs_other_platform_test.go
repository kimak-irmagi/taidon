//go:build !linux

package app

import "testing"

func TestInitLocalBtrfsStorePassesStorePathOnNonLinux(t *testing.T) {
	result, err := initLocalBtrfsStore(localBtrfsInitOptions{StorePath: "/tmp/store"})
	if err != nil {
		t.Fatalf("initLocalBtrfsStore: %v", err)
	}
	if result.StorePath != "/tmp/store" {
		t.Fatalf("unexpected store path: %q", result.StorePath)
	}
}
