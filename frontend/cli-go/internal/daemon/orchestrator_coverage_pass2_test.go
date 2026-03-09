package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func clearCachedEngineStates() {
	cachedEngineStates.mu.Lock()
	cachedEngineStates.m = map[string]EngineState{}
	cachedEngineStates.mu.Unlock()
}

func closeHijackedConnection(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatalf("response writer does not support hijacking")
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		t.Fatalf("Hijack: %v", err)
	}
	_ = conn.Close()
}

func TestCheckAuthWithTokenBranches(t *testing.T) {
	if ok, rejected, reason := checkAuthWithToken(context.Background(), " ", "token", time.Second); ok || rejected || !strings.Contains(reason, "empty endpoint") {
		t.Fatalf("expected empty endpoint rejection, got ok=%v rejected=%v reason=%q", ok, rejected, reason)
	}

	if ok, rejected, reason := checkAuthWithToken(context.Background(), "http://example.com", " ", time.Second); !ok || rejected || reason != "" {
		t.Fatalf("expected empty token to bypass auth probe, got ok=%v rejected=%v reason=%q", ok, rejected, reason)
	}

	if ok, rejected, reason := checkAuthWithToken(context.Background(), "http://bad\nhost", "token", time.Second); ok || rejected || strings.TrimSpace(reason) == "" {
		t.Fatalf("expected request construction error, got ok=%v rejected=%v reason=%q", ok, rejected, reason)
	}

	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := closed.URL
	closed.Close()
	if ok, rejected, reason := checkAuthWithToken(context.Background(), closedURL, "token", 20*time.Millisecond); ok || rejected || strings.TrimSpace(reason) == "" {
		t.Fatalf("expected transport error, got ok=%v rejected=%v reason=%q", ok, rejected, reason)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	if ok, rejected, reason := checkAuthWithToken(context.Background(), server.URL, "token", time.Second); ok || !rejected || !strings.Contains(reason, "401") {
		t.Fatalf("expected auth rejection, got ok=%v rejected=%v reason=%q", ok, rejected, reason)
	}
}

func TestTryHealthyStateBranches(t *testing.T) {
	if _, ok := tryHealthyState(context.Background(), EngineState{Endpoint: "http://127.0.0.1:1"}, 10*time.Millisecond, false); ok {
		t.Fatalf("expected health check failure")
	}

	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-2"}`))
		case "/v1/prepare-jobs":
			_, _ = w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mismatch.Close()

	if _, ok := tryHealthyState(context.Background(), EngineState{Endpoint: mismatch.URL, InstanceID: "inst-1", AuthToken: "token"}, time.Second, false); ok {
		t.Fatalf("expected instance mismatch to be rejected")
	}

	brokenAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			closeHijackedConnection(t, w)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer brokenAuth.Close()

	if _, ok := tryHealthyState(context.Background(), EngineState{Endpoint: brokenAuth.URL, InstanceID: "inst-1", AuthToken: "token"}, time.Second, true); ok {
		t.Fatalf("expected auth transport failure to be rejected")
	}

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			_, _ = w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthy.Close()

	state := EngineState{Endpoint: healthy.URL, InstanceID: "inst-1", AuthToken: "token"}
	if got, ok := tryHealthyState(context.Background(), state, time.Second, false); !ok || got.Endpoint != healthy.URL {
		t.Fatalf("expected healthy cached state, got %+v ok=%v", got, ok)
	}
}

func TestRefreshStateOnAuthRejectedBranches(t *testing.T) {
	nonRejected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			closeHijackedConnection(t, w)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer nonRejected.Close()

	state := EngineState{Endpoint: nonRejected.URL, InstanceID: "inst-1", AuthToken: "token"}
	if _, ok := refreshStateOnAuthRejected(context.Background(), filepath.Join(t.TempDir(), "missing.json"), state, time.Second, true); ok {
		t.Fatalf("expected non-auth-rejected failure")
	}

	rejected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer rejected.Close()

	if _, ok := refreshStateOnAuthRejected(context.Background(), filepath.Join(t.TempDir(), "missing.json"), EngineState{Endpoint: rejected.URL, InstanceID: "inst-1", AuthToken: "token"}, time.Second, true); ok {
		t.Fatalf("expected reread failure after auth rejection")
	}

	stateDir := t.TempDir()
	statePath := filepath.Join(stateDir, "engine.json")
	if err := WriteEngineState(statePath, EngineState{Endpoint: rejected.URL, InstanceID: "inst-1", AuthToken: "token-2"}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}
	if _, ok := refreshStateOnAuthRejected(context.Background(), statePath, EngineState{Endpoint: rejected.URL, InstanceID: "inst-1", AuthToken: "token-1"}, time.Second, true); ok {
		t.Fatalf("expected refreshed auth probe to keep failing")
	}
}

func TestStoreCachedEngineStateSkipsEmptyEndpoint(t *testing.T) {
	clearCachedEngineStates()
	t.Cleanup(clearCachedEngineStates)

	opts := ConnectOptions{StateDir: t.TempDir()}
	storeCachedEngineState(opts, EngineState{})
	if _, ok := loadCachedEngineState(opts); ok {
		t.Fatalf("expected empty endpoint to be ignored")
	}
}

func TestConnectOrStartHealthyAfterLockRecheck(t *testing.T) {
	var healthCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			if atomic.AddInt32(&healthCalls, 1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			_, _ = w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stateDir := t.TempDir()
	if err := WriteEngineState(filepath.Join(stateDir, "engine.json"), EngineState{Endpoint: server.URL, InstanceID: "inst-1"}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}
	result, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     "sqlrs-engine",
		RunDir:         filepath.Join(stateDir, "run"),
		StateDir:       stateDir,
		ClientTimeout:  time.Second,
		StartupTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("ConnectOrStart: %v", err)
	}
	if result.Endpoint != server.URL {
		t.Fatalf("unexpected endpoint: %q", result.Endpoint)
	}
	if got := atomic.LoadInt32(&healthCalls); got < 3 {
		t.Fatalf("expected third health check after lock, got %d", got)
	}
}

func TestConnectOrStartReturnsWSLMountError(t *testing.T) {
	prev := runWSLCommandFn
	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active":
			return "inactive\n", errors.New("inactive")
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "start":
			return "", errors.New("start failed")
		case len(args) >= 1 && args[0] == "journalctl":
			return "mount tail", nil
		default:
			return "", nil
		}
	}
	t.Cleanup(func() { runWSLCommandFn = prev })

	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      false,
		StateDir:       t.TempDir(),
		WSLDistro:      "Ubuntu",
		EngineStoreDir: "/mnt/sqlrs/store",
		WSLMountUnit:   "sqlrs-state-store.mount",
	})
	if err == nil || !strings.Contains(err.Error(), "mount tail") {
		t.Fatalf("expected WSL mount error, got %v", err)
	}
}

func TestConnectOrStartAcquireLockError(t *testing.T) {
	stateDir := t.TempDir()
	runDir := filepath.Join(stateDir, "run")
	if err := os.MkdirAll(filepath.Join(runDir, "daemon.lock"), 0o700); err != nil {
		t.Fatalf("mkdir daemon.lock: %v", err)
	}

	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     "sqlrs-engine",
		RunDir:         runDir,
		StateDir:       stateDir,
		StartupTimeout: time.Second,
	})
	if err == nil {
		t.Fatalf("expected lock acquisition error")
	}
}

func TestConnectOrStartLogsDirErrorAfterAuthRefreshFailure(t *testing.T) {
	stateDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"instanceId":"inst-1"}`))
		case "/v1/prepare-jobs":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := WriteEngineState(filepath.Join(stateDir, "engine.json"), EngineState{Endpoint: server.URL, InstanceID: "inst-1", AuthToken: "token"}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "logs"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write logs file: %v", err)
	}

	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     "sqlrs-engine",
		RunDir:         filepath.Join(stateDir, "run"),
		StateDir:       stateDir,
		ClientTimeout:  time.Second,
		StartupTimeout: time.Second,
		Verbose:        true,
	})
	if err == nil {
		t.Fatalf("expected logs dir error")
	}
}

func TestConnectOrStartWSLLogFileError(t *testing.T) {
	stateDir := t.TempDir()
	logsDir := filepath.Join(stateDir, "logs")
	if err := os.MkdirAll(filepath.Join(logsDir, "engine-wsl.log"), 0o700); err != nil {
		t.Fatalf("mkdir engine-wsl.log: %v", err)
	}

	_, err := ConnectOrStart(context.Background(), ConnectOptions{
		Endpoint:       "auto",
		Autostart:      true,
		DaemonPath:     "sqlrs-engine",
		RunDir:         filepath.Join(stateDir, "run"),
		StateDir:       stateDir,
		WSLDistro:      "Ubuntu",
		StartupTimeout: time.Second,
	})
	if err == nil {
		t.Fatalf("expected WSL log file open error")
	}
}
