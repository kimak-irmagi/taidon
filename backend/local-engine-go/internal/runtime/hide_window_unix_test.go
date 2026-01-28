//go:build !windows

package runtime

import (
	"os/exec"
	"testing"
)

func TestHideWindowNoop(t *testing.T) {
	cmd := exec.Command("true")
	hideWindow(cmd)
}
