package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sqlrs/cli/internal/wsl"
)

func bootstrapWSLInit(ctx context.Context, deps wslInitDeps, opts wslInitOptions) (wslBootstrapPhase, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	logWSLInit(opts.Verbose, "checking WSL availability")
	if _, err := deps.lookPath("wsl.exe"); err != nil {
		return wslBootstrapPhase{}, fmt.Errorf("WSL is not available")
	}

	logWSLInit(opts.Verbose, "listing WSL distros")
	distros, err := deps.listWSLDistros()
	if err != nil {
		return wslBootstrapPhase{}, fmt.Errorf("WSL unavailable: %v", err)
	}
	distro, err := wsl.SelectDistro(distros, opts.Distro)
	if err != nil {
		return wslBootstrapPhase{}, fmt.Errorf("WSL distro resolution failed: %v", err)
	}
	logWSLInit(opts.Verbose, "selected WSL distro: %s", distro)

	if !opts.NoStart {
		if _, err := runWSLCommandFn(ctx, distro, opts.Verbose, "starting WSL distro", "true"); err != nil {
			return wslBootstrapPhase{}, fmt.Errorf("WSL distro start failed: %v", err)
		}
	}

	if err := ensureBtrfsKernel(distro, opts.Verbose); err != nil {
		return wslBootstrapPhase{}, err
	}
	if err := ensureBtrfsProgs(distro, opts.Verbose); err != nil {
		return wslBootstrapPhase{}, err
	}
	if err := ensureNsenter(distro, opts.Verbose); err != nil {
		return wslBootstrapPhase{}, err
	}
	if err := ensureSystemdAvailable(distro, opts.Verbose); err != nil {
		return wslBootstrapPhase{}, err
	}

	warnings := []string{}
	dockerRunning, dockerWarning, err := checkDockerDesktopRunning(opts.Verbose)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Docker Desktop check failed: %v", err))
	} else if !dockerRunning {
		if dockerWarning != "" {
			warnings = append(warnings, dockerWarning)
		}
	} else {
		ok, warn := checkDockerInWSL(distro, opts.Verbose)
		if !ok && warn != "" {
			warnings = append(warnings, warn)
		}
	}

	return wslBootstrapPhase{Distro: distro, Warnings: warnings}, nil
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

func ensureBtrfsKernel(distro string, verbose bool) error {
	out, err := runWSLCommandFn(context.Background(), distro, verbose, "check btrfs kernel", "cat", "/proc/filesystems")
	if err != nil {
		return fmt.Errorf("btrfs kernel check failed: %v", err)
	}
	if !strings.Contains(out, "btrfs") {
		_, _ = runWSLCommandFn(context.Background(), distro, verbose, "load btrfs module (root)", "modprobe", "btrfs")
		out, err = runWSLCommandFn(context.Background(), distro, verbose, "check btrfs kernel", "cat", "/proc/filesystems")
		if err != nil {
			return fmt.Errorf("btrfs kernel check failed: %v", err)
		}
		if !strings.Contains(out, "btrfs") {
			return fmt.Errorf("btrfs kernel support missing")
		}
	}
	return nil
}

func ensureBtrfsProgs(distro string, verbose bool) error {
	_, err := runWSLCommandFn(context.Background(), distro, verbose, "check btrfs-progs", "which", "mkfs.btrfs")
	if err == nil {
		return nil
	}
	logWSLInit(verbose, "installing btrfs-progs")
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(updateCtx, distro, verbose, "apt-get update (root)", "apt-get", "update"); err != nil {
		return fmt.Errorf("btrfs-progs install failed: %v", err)
	}
	installCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(installCtx, distro, verbose, "apt-get install (root)", "apt-get", "install", "-y", "btrfs-progs"); err != nil {
		return fmt.Errorf("btrfs-progs install failed: %v", err)
	}
	return nil
}

func ensureNsenter(distro string, verbose bool) error {
	_, err := runWSLCommandFn(context.Background(), distro, verbose, "check nsenter", "which", "nsenter")
	if err == nil {
		return nil
	}
	logWSLInit(verbose, "installing nsenter (util-linux)")
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(updateCtx, distro, verbose, "apt-get update (root)", "apt-get", "update"); err != nil {
		return fmt.Errorf("nsenter install failed: %v", err)
	}
	installCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(installCtx, distro, verbose, "apt-get install (root)", "apt-get", "install", "-y", "util-linux"); err != nil {
		return fmt.Errorf("nsenter install failed: %v", err)
	}
	return nil
}

func ensureSystemdAvailable(distro string, verbose bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runWSLCommandAllowFailureFn(ctx, distro, verbose, "check systemd (root)", "systemctl", "is-system-running")
	state := strings.TrimSpace(out)
	switch state {
	case "running", "degraded":
		return nil
	}
	if err != nil {
		return fmt.Errorf("systemd is not running in WSL distro %s (enable systemd and restart WSL)", distro)
	}
	if state == "" {
		state = "unknown"
	}
	return fmt.Errorf("systemd is not running in WSL distro %s (state=%s). Enable systemd and restart WSL", distro, state)
}

func checkDockerDesktopRunning(verbose bool) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runHostCommandFn(ctx, verbose, "check docker desktop", "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-Service -Name com.docker.service -ErrorAction SilentlyContinue).Status",
	)
	if err != nil {
		return false, "", err
	}
	status := strings.TrimSpace(out)
	if status == "" {
		if ok := checkDockerPipe(ctx, verbose); ok {
			return true, "", nil
		}
		if ok := checkDockerCLI(ctx, verbose); ok {
			return true, "", nil
		}
		return false, "Docker Desktop is not running (service not found)", nil
	}
	if strings.EqualFold(status, "Running") {
		return true, "", nil
	}
	if ok := checkDockerPipe(ctx, verbose); ok {
		return true, "", nil
	}
	if ok := checkDockerCLI(ctx, verbose); ok {
		return true, "", nil
	}
	return false, "Docker Desktop is not running", nil
}

func checkDockerPipe(ctx context.Context, verbose bool) bool {
	out, err := runHostCommandFn(ctx, verbose, "check docker pipe", "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"[System.IO.File]::Exists('\\\\.\\pipe\\docker_engine')",
	)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(out), "True")
}

func checkDockerCLI(ctx context.Context, verbose bool) bool {
	_, err := runHostCommandFn(ctx, verbose, "check docker cli", "docker", "info")
	return err == nil
}

func checkDockerInWSL(distro string, verbose bool) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := runWSLCommandFn(ctx, distro, verbose, "check docker in WSL", "docker", "info")
	if err == nil {
		return true, ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "command not found"):
		return false, fmt.Sprintf("docker is not installed in WSL distro %s", distro)
	case strings.Contains(msg, "cannot connect to the docker daemon"),
		strings.Contains(msg, "is the docker daemon running"):
		return false, fmt.Sprintf("docker is not available in WSL distro %s. Enable Docker Desktop WSL integration and ensure Docker Desktop is running.", distro)
	default:
		return false, fmt.Sprintf("docker is not available in WSL distro %s: %v", distro, err)
	}
}
