//go:build !windows

package daemon

import (
	"os"
	"testing"
)

func TestProcessExists(t *testing.T) {
	if !processExists(os.Getpid()) {
		t.Fatalf("expected current process to exist")
	}
	if processExists(-1) {
		t.Fatalf("expected negative pid to be false")
	}
}
