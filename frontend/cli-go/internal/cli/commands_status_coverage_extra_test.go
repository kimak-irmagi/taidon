package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunStatusLocalProbeDepsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	withLocalDepsStub(t, func(ctx context.Context, opts LocalDepsOptions) (LocalDepsStatus, error) {
		return LocalDepsStatus{}, errors.New("deps failed")
	})

	_, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "deps failed") {
		t.Fatalf("expected local deps error, got %v", err)
	}
}

func TestPrintStatusSkipsEmptyWarnings(t *testing.T) {
	var out strings.Builder
	PrintStatus(&out, StatusResult{
		OK:       true,
		Endpoint: "http://localhost",
		Profile:  "local",
		Mode:     "local",
		Warnings: []string{"", "warn"},
	})
	got := out.String()
	if strings.Count(got, "warning:") != 1 {
		t.Fatalf("expected one warning line, got %q", got)
	}
}

func TestCheckContainerRuntimeReady(t *testing.T) {
	withExecStubs(t, func(name string) (string, error) {
		if name == "docker" {
			return "docker", nil
		}
		return "", errors.New("unexpected")
	}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 0)
	})

	ok, warn := checkContainerRuntime(context.Background(), "docker", false)
	if !ok || warn != "" {
		t.Fatalf("expected docker ready, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLReady(t *testing.T) {
	withExecStubs(t, func(name string) (string, error) {
		if name == "wsl.exe" {
			return "wsl.exe", nil
		}
		return "", errors.New("unexpected")
	}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 0)
	})

	ok, warn := checkWSL(context.Background(), false)
	if !ok || warn != "" {
		t.Fatalf("expected wsl ready, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsNonVerboseSelectError(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" && len(args) >= 2 && args[0] == "--list" && args[1] == "--verbose" {
			return commandOutput(ctx, "  NAME STATE VERSION\n")
		}
		return exec.CommandContext(ctx, name, args...)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{Verbose: false})
	if ok || warn != "btrfs not ready" {
		t.Fatalf("expected generic btrfs warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsFSMismatch(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific path")
	}
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "nsenter"):
			return commandOutput(ctx, "ext4\n")
		default:
			return commandOutput(ctx, "active\n")
		}
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{
		WSLDistro:    "Ubuntu",
		WSLMountUnit: "sqlrs.mount",
		Verbose:      true,
	})
	if ok || !strings.Contains(warn, "fs=ext4") {
		t.Fatalf("expected fs mismatch warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestStatWSLFSTypeMkdirError(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" && strings.Contains(strings.Join(args, " "), "mkdir -p") {
			return commandExit(ctx, 1)
		}
		return exec.CommandContext(ctx, name, args...)
	})

	_, err := statWSLFSType(context.Background(), "Ubuntu", "/mnt/store")
	if err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestStatWSLFSTypeFallbackError(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "nsenter"):
			if runtime.GOOS == "windows" {
				return exec.CommandContext(ctx, "cmd", "/c", "echo command not found 1>&2 & exit 1")
			}
			return exec.CommandContext(ctx, "sh", "-c", "echo command not found >&2; exit 1")
		default:
			return commandExit(ctx, 1)
		}
	})

	_, err := statWSLFSType(context.Background(), "Ubuntu", "/mnt/store")
	if err == nil {
		t.Fatalf("expected fallback stat error")
	}
}
