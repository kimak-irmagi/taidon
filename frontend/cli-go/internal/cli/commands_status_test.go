package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/daemon"
)

func TestRunStatusRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v1","instanceId":"inst","pid":1}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v1" || result.InstanceID != "inst" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusLocalExplicitEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v2","instanceId":"inst","pid":2}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v2" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"version":"v3","instanceId":"inst","pid":3}`))
	}))
	defer server.Close()

	stateDir := t.TempDir()
	if err := daemon.WriteEngineState(filepath.Join(stateDir, "engine.json"), daemon.EngineState{
		Endpoint:   server.URL,
		AuthToken:  "token",
		InstanceID: "inst",
	}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "local",
		Endpoint: "",
		StateDir: stateDir,
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK || result.Version != "v3" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunStatusRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunStatus(context.Background(), StatusOptions{Mode: "remote"})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunStatusHealthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	if err == nil {
		t.Fatalf("expected health error")
	}
	if result.Endpoint != server.URL {
		t.Fatalf("expected endpoint in result, got %+v", result)
	}
}

func TestRunStatusLocalAutostartDisabled(t *testing.T) {
	_, err := RunStatus(context.Background(), StatusOptions{
		Mode:      "local",
		Endpoint:  "",
		StateDir:  t.TempDir(),
		Autostart: false,
		Timeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "local engine is not running") {
		t.Fatalf("expected autostart error, got %v", err)
	}
}

func TestRunStatusRemoteVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	result, err := RunStatus(context.Background(), StatusOptions{
		Mode:     "remote",
		Endpoint: server.URL,
		Timeout:  time.Second,
		Verbose:  true,
	})
	if err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok result")
	}
}

func TestPrintStatus(t *testing.T) {
	result := StatusResult{
		OK:       true,
		Endpoint: "http://localhost:1",
		Profile:  "local",
		Mode:     "remote",
		Version:  "v1",
		PID:      42,
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "status: ok") || !strings.Contains(out, "version: v1") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "workspace: (none)") {
		t.Fatalf("expected workspace placeholder, got %q", out)
	}
}

func TestPrintStatusIncludesOptionalFields(t *testing.T) {
	result := StatusResult{
		OK:         false,
		Endpoint:   "http://localhost:2",
		Profile:    "remote",
		Mode:       "remote",
		Client:     "v1",
		Workspace:  "/tmp/sqlrs",
		Version:    "engine",
		InstanceID: "instance",
		PID:        7,
	}

	var buf bytes.Buffer
	PrintStatus(&buf, result)
	out := buf.String()
	if !strings.Contains(out, "clientVersion: v1") {
		t.Fatalf("expected clientVersion, got %q", out)
	}
	if !strings.Contains(out, "workspace: /tmp/sqlrs") {
		t.Fatalf("expected workspace, got %q", out)
	}
	if !strings.Contains(out, "instanceId: instance") || !strings.Contains(out, "pid: 7") {
		t.Fatalf("expected instanceId and pid, got %q", out)
	}
}

func TestCheckDockerMissingBinary(t *testing.T) {
	t.Setenv("PATH", "")
	ok, warning := checkDocker(context.Background(), false)
	if ok || warning != "docker not ready" {
		t.Fatalf("expected docker not ready, got ok=%v warning=%q", ok, warning)
	}
	ok, warning = checkDocker(context.Background(), true)
	if ok || !strings.Contains(warning, "docker not ready") {
		t.Fatalf("expected verbose docker warning, got ok=%v warning=%q", ok, warning)
	}
}

func TestCheckWSLMissingBinary(t *testing.T) {
	t.Setenv("PATH", "")
	ok, warning := checkWSL(context.Background(), false)
	if ok || warning != "wsl not ready" {
		t.Fatalf("expected wsl not ready, got ok=%v warning=%q", ok, warning)
	}
	ok, warning = checkWSL(context.Background(), true)
	if ok || !strings.Contains(warning, "wsl not ready") {
		t.Fatalf("expected verbose wsl warning, got ok=%v warning=%q", ok, warning)
	}
}

func TestReadinessLabel(t *testing.T) {
	if readinessLabel(true) != "ok" {
		t.Fatalf("expected ok label")
	}
	if readinessLabel(false) != "missing" {
		t.Fatalf("expected missing label")
	}
}

func TestListWSLDistrosParsesOutput(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" {
			out := "  NAME                   STATE           VERSION\n* Ubuntu                 Running         2\n  Debian                 Stopped         2\n"
			return commandOutput(ctx, out)
		}
		return exec.CommandContext(ctx, name, args...)
	})

	distros, err := listWSLDistros(context.Background())
	if err != nil {
		t.Fatalf("listWSLDistros: %v", err)
	}
	if len(distros) < 2 {
		t.Fatalf("expected parsed distros, got %+v", distros)
	}
}

func TestIsWSLMountUnitActiveExitStatus(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" {
			return commandExit(ctx, 3)
		}
		return exec.CommandContext(ctx, name, args...)
	})

	active, err := isWSLMountUnitActive(context.Background(), "Ubuntu", "sqlrs.mount")
	if err != nil {
		t.Fatalf("isWSLMountUnitActive: %v", err)
	}
	if active {
		t.Fatalf("expected inactive for exit status 3")
	}
}

func TestStatWSLFSTypeSuccess(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "mkdir"):
			return commandExit(ctx, 0)
		case strings.Contains(joined, "nsenter"):
			return commandOutput(ctx, "btrfs\n")
		default:
			return commandOutput(ctx, "btrfs\n")
		}
	})

	fs, err := statWSLFSType(context.Background(), "Ubuntu", "/mnt/store")
	if err != nil {
		t.Fatalf("statWSLFSType: %v", err)
	}
	if strings.TrimSpace(fs) != "btrfs" {
		t.Fatalf("expected btrfs, got %q", fs)
	}
}

func TestCheckWSLBtrfsOk(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != "wsl.exe" {
			return exec.CommandContext(ctx, name, args...)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "systemctl"):
			return commandOutput(ctx, "active\n")
		case strings.Contains(joined, "mkdir"):
			return commandExit(ctx, 0)
		default:
			return commandOutput(ctx, "btrfs\n")
		}
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{
		WSLDistro:      "Ubuntu",
		WSLStateDir:    "/mnt/store",
		WSLMountUnit:   "sqlrs.mount",
		WSLMountFSType: "btrfs",
		Verbose:        false,
	})
	if !ok || warn != "" {
		t.Fatalf("expected btrfs ok, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckDockerCommandFailure(t *testing.T) {
	withExecStubs(t, func(name string) (string, error) {
		return "docker", nil
	}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 1)
	})

	ok, warning := checkDocker(context.Background(), false)
	if ok || warning != "docker not ready" {
		t.Fatalf("expected docker not ready, got ok=%v warning=%q", ok, warning)
	}
	ok, warning = checkDocker(context.Background(), true)
	if ok || !strings.Contains(warning, "docker not ready") {
		t.Fatalf("expected verbose warning, got ok=%v warning=%q", ok, warning)
	}
}

func TestCheckWSLCommandFailure(t *testing.T) {
	withExecStubs(t, func(name string) (string, error) {
		return "wsl.exe", nil
	}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return commandExit(ctx, 1)
	})

	ok, warning := checkWSL(context.Background(), false)
	if ok || warning != "wsl not ready" {
		t.Fatalf("expected wsl not ready, got ok=%v warning=%q", ok, warning)
	}
	ok, warning = checkWSL(context.Background(), true)
	if ok || !strings.Contains(warning, "wsl not ready") {
		t.Fatalf("expected verbose warning, got ok=%v warning=%q", ok, warning)
	}
}

func TestCheckWSLBtrfsListDistroError(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" {
			return commandExit(ctx, 1)
		}
		return exec.CommandContext(ctx, name, args...)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{Verbose: true})
	if ok || warn == "" {
		t.Fatalf("expected btrfs warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestCheckWSLBtrfsInactiveMountUnit(t *testing.T) {
	withExecStubs(t, nil, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "wsl.exe" {
			return commandExit(ctx, 1)
		}
		return exec.CommandContext(ctx, name, args...)
	})

	ok, warn := checkWSLBtrfs(context.Background(), LocalDepsOptions{
		WSLDistro:    "Ubuntu",
		WSLMountUnit: "sqlrs.mount",
		Verbose:      true,
	})
	if ok || warn == "" {
		t.Fatalf("expected btrfs warning, got ok=%v warn=%q", ok, warn)
	}
}

func TestIsExitStatus(t *testing.T) {
	err := runExitStatusError(4)
	if !isExitStatus(err, 4) {
		t.Fatalf("expected exit status 4 to match")
	}
	if isExitStatus(err, 2) {
		t.Fatalf("expected exit status mismatch")
	}
	if isExitStatus(fmt.Errorf("other"), 1) {
		t.Fatalf("expected non-exit error to return false")
	}
}

func withExecStubs(t *testing.T, lookPath func(string) (string, error), cmd func(context.Context, string, ...string) *exec.Cmd) {
	t.Helper()
	prevLook := execLookPathFn
	prevCmd := execCommandContextFn
	if lookPath != nil {
		execLookPathFn = lookPath
	}
	if cmd != nil {
		execCommandContextFn = cmd
	}
	t.Cleanup(func() {
		execLookPathFn = prevLook
		execCommandContextFn = prevCmd
	})
}

func commandOutput(ctx context.Context, output string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
		parts := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				parts = append(parts, "echo.")
				continue
			}
			parts = append(parts, "echo "+line)
		}
		return exec.CommandContext(ctx, "cmd", "/c", strings.Join(parts, " & "))
	}
	escaped := strings.ReplaceAll(output, `"`, `\"`)
	return exec.CommandContext(ctx, "sh", "-c", "printf \"%s\" \""+escaped+"\"")
}

func commandExit(ctx context.Context, code int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", fmt.Sprintf("exit %d", code))
	}
	return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("exit %d", code))
}

func runExitStatusError(code int) error {
	cmd := commandExit(context.Background(), code)
	return cmd.Run()
}
