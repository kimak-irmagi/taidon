//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"testing"
)

func TestProcessExists(t *testing.T) {
	if processExists(-1) {
		t.Fatalf("expected false for negative pid")
	}
	if !processExists(os.Getpid()) {
		t.Fatalf("expected current pid to exist")
	}
}

func TestConfigureDetached(t *testing.T) {
	cmd := exec.Command("cmd")
	configureDetached(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
}
