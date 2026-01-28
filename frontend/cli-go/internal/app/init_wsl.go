package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"sqlrs/cli/internal/wsl"
)

type wslInitOptions struct {
	Enable    bool
	Distro    string
	Require   bool
	NoStart   bool
	Workspace string
}

type wslInitResult struct {
	UseWSL     bool
	Distro     string
	StateDir   string
	EnginePath string
	Warning    string
}

var initWSLFn = initWSL

const defaultWSLStateDir = "~/.local/state/sqlrs"
const defaultWSLBtrfsImage = "~/.local/share/sqlrs/btrfs.img"
const defaultWSLBtrfsSize = "10G"

func initWSL(opts wslInitOptions) (wslInitResult, error) {
	if !opts.Enable {
		return wslInitResult{}, nil
	}
	if runtime.GOOS != "windows" {
		return wslInitResult{}, fmt.Errorf("WSL init is only supported on Windows")
	}

	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return wslUnavailable(opts, "WSL is not available")
	}

	distros, err := listWSLDistros()
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL unavailable: %v", err))
	}
	distro, err := wsl.SelectDistro(distros, opts.Distro)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL distro resolution failed: %v", err))
	}

	if !opts.NoStart {
		if err := startWSLDistro(distro); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("WSL distro start failed: %v", err))
		}
	}

	stateDir := defaultWSLStateDir
	if err := ensureBtrfsStateDir(distro, stateDir); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs not ready: %v", err))
	}

	return wslInitResult{
		UseWSL:   true,
		Distro:   distro,
		StateDir: stateDir,
	}, nil
}

func listWSLDistros() ([]wsl.Distro, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", "--verbose")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return wsl.ParseDistroList(string(out))
}

func startWSLDistro(distro string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "echo", "ready")
	return cmd.Run()
}

func ensureBtrfsStateDir(distro, stateDir string) error {
	fsType, err := statWSLFSType(distro, stateDir)
	if err == nil && strings.TrimSpace(fsType) == "btrfs" {
		return nil
	}
	if err := initWSLBtrfsVolume(distro, stateDir); err != nil {
		return err
	}
	fsType, err = statWSLFSType(distro, stateDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(fsType) != "btrfs" {
		return fmt.Errorf("expected btrfs, got %s", strings.TrimSpace(fsType))
	}
	return nil
}

func statWSLFSType(distro, stateDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "sh", "-lc",
		fmt.Sprintf("mkdir -p %s && stat -f -c %%T %s", stateDir, stateDir))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func initWSLBtrfsVolume(distro, stateDir string) error {
	script := strings.Join([]string{
		"set -euo pipefail",
		fmt.Sprintf("img=%s", defaultWSLBtrfsImage),
		"mnt=" + stateDir,
		"mkdir -p \"$(dirname \"$img\")\" \"$mnt\"",
		"if [ ! -f \"$img\" ]; then fallocate -l " + defaultWSLBtrfsSize + " \"$img\"; mkfs.btrfs -f \"$img\"; fi",
		"if ! mountpoint -q \"$mnt\"; then sudo mount -o loop \"$img\" \"$mnt\"; fi",
	}, "; ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "sh", "-lc", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("btrfs init failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func wslUnavailable(opts wslInitOptions, warning string) (wslInitResult, error) {
	if opts.Require {
		return wslInitResult{}, errors.New(warning)
	}
	return wslInitResult{UseWSL: false, Warning: warning}, nil
}
