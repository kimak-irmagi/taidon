package cli

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestProbeLocalDepsWSLReadyBtrfsWarningCoverage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific path")
	}

	withExecStubs(t, func(name string) (string, error) {
		switch name {
		case "docker":
			return "docker", nil
		case "wsl.exe":
			return "wsl.exe", nil
		default:
			return "", errors.New("unexpected lookpath")
		}
	}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "docker" && name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case name == "docker":
			return commandExit(ctx, 0)
		case strings.Contains(joined, "--status"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "mkdir -p"):
			return commandExit(ctx, 1)
		default:
			return commandExit(ctx, 0)
		}
	})

	status, err := probeLocalDeps(context.Background(), LocalDepsOptions{WSLDistro: "Ubuntu", Verbose: true})
	if err != nil {
		t.Fatalf("probeLocalDeps: %v", err)
	}
	if !status.DockerReady || !status.WSLReady || status.BtrfsReady {
		t.Fatalf("unexpected local deps: %+v", status)
	}
	if len(status.Warnings) == 0 {
		t.Fatalf("expected btrfs warning")
	}
}

func TestCheckWSLBtrfsVerboseSelectErrorCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" && len(args) >= 2 && args[0] == "--list" && args[1] == "--verbose" {
			return commandOutput(ctx, "  NAME STATE VERSION\n")
		}
		return exec.CommandContext(ctx, name, args...)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{Verbose: true})
	if ok || !strings.Contains(strings.ToLower(warn), "btrfs not ready") {
		t.Fatalf("expected verbose select error warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsNonVerboseSelectErrorCoverage(t *testing.T) {
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

func TestCheckWSLBtrfsInactiveMountUnitCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "systemctl is-active") {
			return commandExit(ctx, 3)
		}
		return commandExit(ctx, 0)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{
		WSLDistro:    "Ubuntu",
		WSLMountUnit: "sqlrs.mount",
		Verbose:      true,
	})
	if ok || warn != "btrfs not ready" {
		t.Fatalf("expected inactive mount unit warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsStatErrorNonVerboseCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		if strings.Contains(strings.Join(args, " "), "mkdir -p") {
			return commandExit(ctx, 1)
		}
		return commandExit(ctx, 0)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{WSLDistro: "Ubuntu", Verbose: false})
	if ok || warn != "btrfs not ready" {
		t.Fatalf("expected generic stat warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsFSMismatchNonVerboseCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir -p"):
			return commandExit(ctx, 0)
		default:
			return commandOutput(ctx, "ext4\n")
		}
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{WSLDistro: "Ubuntu", Verbose: false})
	if ok || warn != "btrfs not ready" {
		t.Fatalf("expected fs mismatch generic warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestStatWSLFSTypeFallbackSuccessCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir -p"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "nsenter"):
			return exec.CommandContext(ctx, "command not found")
		default:
			return commandOutput(ctx, "btrfs\n")
		}
	})

	fs, err := statWSLFSType(context.Background(), "Ubuntu", "/mnt/store")
	if err != nil {
		t.Fatalf("statWSLFSType: %v", err)
	}
	if strings.TrimSpace(fs) != "btrfs" {
		t.Fatalf("expected btrfs from fallback stat, got %q", fs)
	}
}

func TestStatWSLFSTypeFallbackStatErrorCoverage(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir -p"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "nsenter"):
			return exec.CommandContext(ctx, "command not found")
		default:
			return commandExit(ctx, 1)
		}
	})

	if _, err := statWSLFSType(context.Background(), "Ubuntu", "/mnt/store"); err == nil {
		t.Fatalf("expected fallback stat error")
	}
}
