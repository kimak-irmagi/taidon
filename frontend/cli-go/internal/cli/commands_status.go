package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
	"sqlrs/cli/internal/wsl"
)

type StatusOptions struct {
	ProfileName     string
	Mode            string
	AuthToken       string
	Endpoint        string
	Autostart       bool
	DaemonPath      string
	RunDir          string
	StateDir        string
	EngineRunDir    string
	EngineStatePath string
	EngineStoreDir  string
	WSLVHDXPath     string
	WSLMountUnit    string
	WSLMountFSType  string
	WSLDistro       string
	Timeout         time.Duration
	IdleTimeout     time.Duration
	StartupTimeout  time.Duration
	Verbose         bool
}

type StatusResult struct {
	OK         bool   `json:"ok"`
	Endpoint   string `json:"endpoint"`
	Profile    string `json:"profile"`
	Mode       string `json:"mode"`
	Client     string `json:"clientVersion,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	Version    string `json:"version,omitempty"`
	InstanceID string `json:"instanceId,omitempty"`
	PID        int    `json:"pid,omitempty"`
	// DockerReady is kept for backward-compatible JSON shape.
	// It reports container runtime readiness (docker or podman).
	DockerReady bool     `json:"dockerReady,omitempty"`
	WSLReady    bool     `json:"wslReady,omitempty"`
	BtrfsReady  bool     `json:"btrfsReady,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type LocalDepsOptions struct {
	Verbose        bool
	WSLDistro      string
	WSLStateDir    string
	WSLMountUnit   string
	WSLMountFSType string
	RuntimeMode    string
}

type LocalDepsStatus struct {
	// DockerReady reports container runtime readiness (docker or podman).
	DockerReady bool
	WSLReady    bool
	BtrfsReady  bool
	Warnings    []string
}

var probeLocalDepsFn = probeLocalDeps
var execLookPathFn = exec.LookPath
var execCommandContextFn = exec.CommandContext

func RunStatus(ctx context.Context, opts StatusOptions) (StatusResult, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	endpoint := strings.TrimSpace(opts.Endpoint)
	authToken := strings.TrimSpace(opts.AuthToken)

	if mode == "local" {
		authToken = ""
		if endpoint == "" {
			endpoint = "auto"
		}
		if endpoint == "auto" {
			if opts.Verbose {
				fmt.Fprintln(os.Stderr, "checking local engine state")
			}
			resolved, err := daemon.ConnectOrStart(ctx, daemon.ConnectOptions{
				Endpoint:        endpoint,
				Autostart:       opts.Autostart,
				DaemonPath:      opts.DaemonPath,
				RunDir:          opts.RunDir,
				StateDir:        opts.StateDir,
				EngineRunDir:    opts.EngineRunDir,
				EngineStatePath: opts.EngineStatePath,
				EngineStoreDir:  opts.EngineStoreDir,
				WSLVHDXPath:     opts.WSLVHDXPath,
				WSLMountUnit:    opts.WSLMountUnit,
				WSLMountFSType:  opts.WSLMountFSType,
				WSLDistro:       opts.WSLDistro,
				IdleTimeout:     opts.IdleTimeout,
				StartupTimeout:  opts.StartupTimeout,
				ClientTimeout:   opts.Timeout,
				Verbose:         opts.Verbose,
			})
			if err != nil {
				return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, err
			}
			endpoint = resolved.Endpoint
			authToken = resolved.AuthToken
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: authToken})
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, "requesting health")
	}
	health, err := cliClient.Health(ctx)
	if err != nil {
		return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, err
	}

	var deps LocalDepsStatus
	if mode == "local" {
		runtimeMode := runtimeModeFromEngineConfig(ctx, cliClient, opts.Verbose)
		deps, err = probeLocalDepsFn(ctx, LocalDepsOptions{
			Verbose:        opts.Verbose,
			WSLDistro:      opts.WSLDistro,
			WSLStateDir:    opts.EngineStoreDir,
			WSLMountUnit:   opts.WSLMountUnit,
			WSLMountFSType: opts.WSLMountFSType,
			RuntimeMode:    runtimeMode,
		})
		if err != nil {
			return StatusResult{Endpoint: endpoint, Profile: opts.ProfileName, Mode: mode}, err
		}
		for _, warning := range deps.Warnings {
			if warning != "" {
				fmt.Fprintln(os.Stderr, warning)
			}
		}
	}

	return StatusResult{
		OK:          health.Ok,
		Endpoint:    endpoint,
		Profile:     opts.ProfileName,
		Mode:        mode,
		Version:     health.Version,
		InstanceID:  health.InstanceID,
		PID:         health.PID,
		DockerReady: deps.DockerReady,
		WSLReady:    deps.WSLReady,
		BtrfsReady:  deps.BtrfsReady,
		Warnings:    deps.Warnings,
	}, nil
}

func PrintStatus(w io.Writer, result StatusResult) {
	status := "unavailable"
	if result.OK {
		status = "ok"
	}

	fmt.Fprintf(w, "status: %s\n", status)
	fmt.Fprintf(w, "endpoint: %s\n", result.Endpoint)
	fmt.Fprintf(w, "profile: %s\n", result.Profile)
	fmt.Fprintf(w, "mode: %s\n", result.Mode)
	if result.Client != "" {
		fmt.Fprintf(w, "clientVersion: %s\n", result.Client)
	}
	if result.Workspace != "" {
		fmt.Fprintf(w, "workspace: %s\n", result.Workspace)
	} else {
		fmt.Fprintln(w, "workspace: (none)")
	}

	if result.Version != "" {
		fmt.Fprintf(w, "version: %s\n", result.Version)
	}
	if result.InstanceID != "" {
		fmt.Fprintf(w, "instanceId: %s\n", result.InstanceID)
	}
	if result.PID != 0 {
		fmt.Fprintf(w, "pid: %d\n", result.PID)
	}
	if result.Mode == "local" {
		fmt.Fprintf(w, "container-runtime: %s\n", readinessLabel(result.DockerReady))
		fmt.Fprintf(w, "wsl: %s\n", readinessLabel(result.WSLReady))
		fmt.Fprintf(w, "btrfs: %s\n", readinessLabel(result.BtrfsReady))
	}
	for _, warning := range result.Warnings {
		if warning == "" {
			continue
		}
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

func readinessLabel(ok bool) string {
	if ok {
		return "ok"
	}
	return "missing"
}

func probeLocalDeps(ctx context.Context, opts LocalDepsOptions) (LocalDepsStatus, error) {
	status := LocalDepsStatus{}
	var warnings []string

	containerReady, containerWarning := checkContainerRuntime(ctx, opts.RuntimeMode, opts.Verbose)
	status.DockerReady = containerReady
	if containerWarning != "" {
		warnings = append(warnings, containerWarning)
	}

	if runtime.GOOS == "windows" {
		wslReady, wslWarning := checkWSL(ctx, opts.Verbose)
		status.WSLReady = wslReady
		if wslWarning != "" {
			warnings = append(warnings, wslWarning)
		}
		if wslReady {
			btrfsReady, btrfsWarning := checkWSLBtrfs(ctx, opts)
			status.BtrfsReady = btrfsReady
			if btrfsWarning != "" {
				warnings = append(warnings, btrfsWarning)
			}
		}
	}

	status.Warnings = warnings
	return status, nil
}

const defaultWSLStateDir = "~/.local/state/sqlrs/store"

type runtimeProbe struct {
	name string
	path string
	err  error
}

// checkContainerRuntime verifies that a supported container runtime is available and responsive.
// Runtime check order matches the engine runtime selection:
// - SQLRS_CONTAINER_RUNTIME override, if set
// - otherwise container.runtime mode (docker|podman|auto)
// - auto probes docker then podman
func checkContainerRuntime(ctx context.Context, mode string, verbose bool) (bool, string) {
	candidates, source := runtimeProbeCandidates(mode)
	probes := make([]runtimeProbe, 0, 2)
	for _, name := range candidates {
		path, err := execLookPathFn(name)
		if err != nil {
			probes = append(probes, runtimeProbe{name: name, err: fmt.Errorf("not found: %w", err)})
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := execCommandContextFn(checkCtx, path, "info")
		err = cmd.Run()
		cancel()
		if err == nil {
			return true, ""
		}
		probes = append(probes, runtimeProbe{name: name, path: path, err: fmt.Errorf("info failed: %w", err)})
	}
	if verbose {
		return false, formatContainerRuntimeProbeWarning(source, candidates, probes)
	}
	return false, "container runtime not ready (docker/podman)"
}

func formatContainerRuntimeProbeWarning(source string, candidates []string, probes []runtimeProbe) string {
	if len(probes) == 0 {
		return fmt.Sprintf("container runtime not ready: source=%s; checked=%s; no probe results", source, strings.Join(candidates, " -> "))
	}
	details := make([]string, 0, len(probes))
	for _, probe := range probes {
		target := probe.name
		if probe.path != "" && probe.path != probe.name {
			target = fmt.Sprintf("%s (%s)", probe.name, probe.path)
		}
		if probe.err == nil {
			details = append(details, target+": ready")
			continue
		}
		details = append(details, fmt.Sprintf("%s: %v", target, probe.err))
	}
	return fmt.Sprintf("container runtime not ready: source=%s; checked=%s; %s", source, strings.Join(candidates, " -> "), strings.Join(details, "; "))
}

func runtimeProbeCandidates(mode string) ([]string, string) {
	if override := strings.TrimSpace(os.Getenv("SQLRS_CONTAINER_RUNTIME")); override != "" {
		name := strings.TrimSpace(override)
		if filepath.Base(name) != "." {
			name = filepath.Base(name)
		}
		return []string{override}, "env SQLRS_CONTAINER_RUNTIME=" + name
	}
	mode = normalizeContainerRuntimeMode(mode)
	switch mode {
	case "docker":
		return []string{"docker"}, "config container.runtime=docker"
	case "podman":
		return []string{"podman"}, "config container.runtime=podman"
	default:
		return []string{"docker", "podman"}, "config container.runtime=auto"
	}
}

func runtimeModeFromEngineConfig(ctx context.Context, cliClient *client.Client, verbose bool) string {
	value, err := cliClient.GetConfig(ctx, "container.runtime", true)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "container runtime mode probe failed: %v\n", err)
		}
		return "auto"
	}
	entry, ok := value.(client.ConfigValue)
	if !ok {
		return "auto"
	}
	mode, ok := entry.Value.(string)
	if !ok {
		return "auto"
	}
	return normalizeContainerRuntimeMode(mode)
}

func normalizeContainerRuntimeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "docker":
		return "docker"
	case "podman":
		return "podman"
	default:
		return "auto"
	}
}

func checkWSL(ctx context.Context, verbose bool) (bool, string) {
	if _, err := execLookPathFn("wsl.exe"); err != nil {
		if verbose {
			return false, fmt.Sprintf("wsl not ready: %v", err)
		}
		return false, "wsl not ready"
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := execCommandContextFn(checkCtx, "wsl.exe", "--status")
	if err := cmd.Run(); err != nil {
		if verbose {
			return false, fmt.Sprintf("wsl not ready: %v", err)
		}
		return false, "wsl not ready"
	}
	return true, ""
}

func checkWSLBtrfs(ctx context.Context, opts LocalDepsOptions) (bool, string) {
	distro := strings.TrimSpace(opts.WSLDistro)
	if distro == "" {
		distros, err := listWSLDistros(ctx)
		if err != nil {
			if opts.Verbose {
				return false, fmt.Sprintf("btrfs not ready: %v", err)
			}
			return false, "btrfs not ready"
		}
		distro, err = wsl.SelectDistro(distros, "")
		if err != nil {
			if opts.Verbose {
				return false, fmt.Sprintf("btrfs not ready: %v", err)
			}
			return false, "btrfs not ready"
		}
	}
	stateDir := strings.TrimSpace(opts.WSLStateDir)
	if stateDir == "" {
		stateDir = defaultWSLStateDir
	}
	unit := strings.TrimSpace(opts.WSLMountUnit)
	if unit != "" {
		active, err := isWSLMountUnitActive(ctx, distro, unit)
		if err != nil || !active {
			if opts.Verbose && err != nil {
				return false, fmt.Sprintf("btrfs not ready: %v", err)
			}
			return false, "btrfs not ready"
		}
	}
	fsType, err := statWSLFSType(ctx, distro, stateDir)
	if err != nil {
		if opts.Verbose {
			return false, fmt.Sprintf("btrfs not ready: %v", err)
		}
		return false, "btrfs not ready"
	}
	if strings.TrimSpace(fsType) != "btrfs" {
		if opts.Verbose {
			return false, fmt.Sprintf("btrfs not ready: fs=%s", strings.TrimSpace(fsType))
		}
		return false, "btrfs not ready"
	}
	return true, ""
}

func listWSLDistros(ctx context.Context) ([]wsl.Distro, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := execCommandContextFn(checkCtx, "wsl.exe", "--list", "--verbose")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return wsl.ParseDistroList(string(out))
}

func isWSLMountUnitActive(ctx context.Context, distro, unit string) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := execCommandContextFn(checkCtx, "wsl.exe", "-d", distro, "-u", "root", "--", "systemctl", "is-active", unit)
	out, err := cmd.Output()
	if err != nil {
		if isExitStatus(err, 3) || isExitStatus(err, 4) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) == "active", nil
}

func statWSLFSType(ctx context.Context, distro, stateDir string) (string, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := execCommandContextFn(checkCtx, "wsl.exe", "-d", distro, "-u", "root", "--", "mkdir", "-p", stateDir)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	cmd = execCommandContextFn(checkCtx, "wsl.exe", "-d", distro, "-u", "root", "--", "nsenter", "-t", "1", "-m", "--", "stat", "-f", "-c", "%T", stateDir)
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "command not found") {
		cmd = execCommandContextFn(checkCtx, "wsl.exe", "-d", distro, "-u", "root", "--", "stat", "-f", "-c", "%T", stateDir)
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return "", err
}

func isExitStatus(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}
