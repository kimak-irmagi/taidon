//go:build !windows

package runtime

import (
	"os/exec"
	"testing"
)

func TestHideWindowNoop(t *testing.T) {
	cmd := exec.Command("sh", "-c", "true")
	hideWindow(cmd)
}
