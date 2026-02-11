package daemon

import (
	"context"
	"errors"
	"os/exec"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestEnsureWSLStoreMountAttachRetryMismatch(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path")
	}
	prevWSL := runWSLCommandFn
	prevHost := runHostCommandFn
	t.Cleanup(func() {
		runWSLCommandFn = prevWSL
		runHostCommandFn = prevHost
	})

	runHostCommandFn = func(ctx context.Context, args ...string) (string, error) {
		return "", nil
	}

	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active":
			return "active\n", nil
		case len(args) >= 1 && args[0] == "nsenter":
			return "ext4\n", nil
		case len(args) >= 1 && args[0] == "journalctl":
			return "tail", nil
		default:
			return "", nil
		}
	}

	err := ensureWSLStoreMount(context.Background(), ConnectOptions{
		WSLDistro:      "Ubuntu",
		EngineStoreDir: "/mnt/sqlrs/store",
		WSLMountUnit:   "sqlrs-state-store.mount",
		WSLVHDXPath:    "C:\\temp\\store.vhdx",
	})
	if err == nil || !strings.Contains(err.Error(), "expected btrfs") {
		t.Fatalf("expected mount mismatch error, got %v", err)
	}
}

func TestEnsureWSLStoreMountAttachError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path")
	}
	prevWSL := runWSLCommandFn
	prevHost := runHostCommandFn
	t.Cleanup(func() {
		runWSLCommandFn = prevWSL
		runHostCommandFn = prevHost
	})

	runHostCommandFn = func(ctx context.Context, args ...string) (string, error) {
		return "", errors.New("attach failed")
	}

	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active":
			return "inactive\n", errors.New("inactive")
		case len(args) >= 1 && args[0] == "journalctl":
			return "tail", nil
		default:
			return "", errors.New("boom")
		}
	}

	err := ensureWSLStoreMount(context.Background(), ConnectOptions{
		WSLDistro:      "Ubuntu",
		EngineStoreDir: "/mnt/sqlrs/store",
		WSLMountUnit:   "sqlrs-state-store.mount",
		WSLVHDXPath:    "C:\\temp\\store.vhdx",
	})
	if err == nil || !strings.Contains(err.Error(), "attach failed") {
		t.Fatalf("expected attach error, got %v", err)
	}
}

func TestEnsureWSLStoreMountEmptyFSType(t *testing.T) {
	prev := runWSLCommandFn
	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active" {
			return "active\n", nil
		}
		return "\n", nil
	}
	t.Cleanup(func() { runWSLCommandFn = prev })

	err := ensureWSLStoreMount(context.Background(), ConnectOptions{
		WSLDistro:      "Ubuntu",
		EngineStoreDir: "/mnt/sqlrs/store",
		WSLMountUnit:   "sqlrs-state-store.mount",
		WSLMountFSType: "btrfs",
	})
	if err == nil || !strings.Contains(err.Error(), "empty fstype") {
		t.Fatalf("expected empty fstype error, got %v", err)
	}
}

func TestRunWSLCommandInInitNamespaceFallback(t *testing.T) {
	prev := runWSLCommandFn
	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "nsenter" {
			return "", errors.New("command not found")
		}
		return "ok", nil
	}
	t.Cleanup(func() { runWSLCommandFn = prev })

	out, err := runWSLCommandInInitNamespace(context.Background(), "Ubuntu", "echo", "hi")
	if err != nil || strings.TrimSpace(out) != "ok" {
		t.Fatalf("expected fallback output, got %q err=%v", out, err)
	}
}

func TestEnsureWSLMountUnitActiveStartError(t *testing.T) {
	prev := runWSLCommandFn
	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active":
			return "inactive\n", errors.New("inactive")
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "start":
			return "", errors.New("start failed")
		case len(args) >= 1 && args[0] == "journalctl":
			return "tail", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runWSLCommandFn = prev })

	err := ensureWSLMountUnitActive(context.Background(), "Ubuntu", "unit.mount")
	if err == nil || !strings.Contains(err.Error(), "tail") {
		t.Fatalf("expected mount unit error with logs, got %v", err)
	}
}

func TestAttachVHDXToWSLErrorNoOutput(t *testing.T) {
	prev := runHostCommandFn
	runHostCommandFn = func(ctx context.Context, args ...string) (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() { runHostCommandFn = prev })

	err := attachVHDXToWSL(nil, "C:\\temp\\store.vhdx", false)
	if err == nil || !strings.Contains(err.Error(), "attach VHDX failed") {
		t.Fatalf("expected attach error, got %v", err)
	}
}

func TestRunWSLCommandNilContext(t *testing.T) {
	if _, err := runWSLCommand(nil, "missing-distro", "true"); err == nil {
		t.Fatalf("expected runWSLCommand error")
	}
}

func TestRunHostCommandNilContext(t *testing.T) {
	var out string
	var err error
	if _, lookErr := exec.LookPath("cmd.exe"); lookErr == nil {
		out, err = runHostCommand(nil, "cmd.exe", "/c", "echo", "ok")
	} else {
		out, err = runHostCommand(nil, "sh", "-c", "echo ok")
	}
	if err != nil || !strings.Contains(out, "ok") {
		t.Fatalf("expected host command output, got %q err=%v", out, err)
	}
}

func TestLogTailStopNil(t *testing.T) {
	var tail *logTail
	tail.Stop()
}

func TestStartLogTailOpenError(t *testing.T) {
	if _, err := startLogTail(filepath.Join(t.TempDir(), "missing.log"), io.Discard); err == nil {
		t.Fatalf("expected open error")
	}
}

func TestLoadHealthyStateWithReasonInstanceMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"instanceId":"inst-2"}`)
	}))
	t.Cleanup(server.Close)

	temp := t.TempDir()
	statePath := filepath.Join(temp, "engine.json")
	state := EngineState{Endpoint: server.URL, InstanceID: "inst-1"}
	if err := WriteEngineState(statePath, state); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	_, ok, reason := loadHealthyStateWithReason(context.Background(), statePath, time.Second)
	if ok || !strings.Contains(reason, "instanceId mismatch") {
		t.Fatalf("expected instanceId mismatch, got ok=%v reason=%q", ok, reason)
	}
}

func TestLoadHealthyStateWithReasonPidNotRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
	}))
	t.Cleanup(server.Close)

	temp := t.TempDir()
	statePath := filepath.Join(temp, "engine.json")
	state := EngineState{Endpoint: server.URL, InstanceID: "inst", PID: 123}
	if err := WriteEngineState(statePath, state); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	prev := isWindows
	isWindows = false
	t.Cleanup(func() { isWindows = prev })

	_, ok, reason := loadHealthyStateWithReason(context.Background(), statePath, time.Second)
	if ok || !strings.Contains(reason, "engine pid not running") {
		t.Fatalf("expected pid not running reason, got ok=%v reason=%q", ok, reason)
	}
}
