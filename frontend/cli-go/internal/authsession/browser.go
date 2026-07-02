package authsession

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var startBrowserCommand = func(cmd *exec.Cmd) error {
	return cmd.Start()
}

// OpenBrowser launches the system browser for the Google authorization URL.
func OpenBrowser(ctx context.Context, authURL string) error {
	authURL = strings.TrimSpace(authURL)
	if authURL == "" {
		return fmt.Errorf("authorization URL is empty")
	}
	name, args := browserCommand(runtime.GOOS, authURL)
	return startBrowserCommand(exec.CommandContext(ctx, name, args...))
}

func browserCommand(goos string, authURL string) (string, []string) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", authURL}
	case "darwin":
		return "open", []string{authURL}
	default:
		return "xdg-open", []string{authURL}
	}
}
