package authsession

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestOpenBrowserRejectsEmptyURL(t *testing.T) {
	if err := OpenBrowser(context.Background(), "  "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty URL error, got %v", err)
	}
}

func TestOpenBrowserStartsPlatformCommand(t *testing.T) {
	var captured *exec.Cmd
	oldStart := startBrowserCommand
	startBrowserCommand = func(cmd *exec.Cmd) error {
		captured = cmd
		return nil
	}
	t.Cleanup(func() { startBrowserCommand = oldStart })

	if err := OpenBrowser(context.Background(), "https://accounts.google.com/auth"); err != nil {
		t.Fatalf("OpenBrowser: %v", err)
	}
	if captured == nil {
		t.Fatalf("expected command to be started")
	}
	if got := strings.Join(captured.Args, " "); !strings.Contains(got, "https://accounts.google.com/auth") {
		t.Fatalf("command args = %q, want auth URL", got)
	}
}

func TestBrowserCommandSelectsPlatformLauncher(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: "rundll32"},
		{goos: "darwin", want: "open"},
		{goos: "linux", want: "xdg-open"},
	}
	for _, tc := range cases {
		name, args := browserCommand(tc.goos, "https://example.com")
		if name != tc.want {
			t.Fatalf("browserCommand(%s) name=%q want %q", tc.goos, name, tc.want)
		}
		if got := strings.Join(args, " "); !strings.Contains(got, "https://example.com") {
			t.Fatalf("browserCommand(%s) args=%q", tc.goos, got)
		}
	}
}
