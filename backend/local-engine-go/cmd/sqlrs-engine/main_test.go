package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestActivityTrackerIdleFor(t *testing.T) {
	tracker := newActivityTracker()
	tracker.last = time.Now().Add(-2 * time.Second).UnixNano()
	if tracker.IdleFor() < time.Second {
		t.Fatalf("expected idle >= 1s, got %v", tracker.IdleFor())
	}
	tracker.Touch()
	if tracker.IdleFor() < 0 {
		t.Fatalf("expected non-negative idle")
	}
}

func TestRandomHex(t *testing.T) {
	value, err := randomHex(8)
	if err != nil {
		t.Fatalf("randomHex: %v", err)
	}
	if len(value) != 16 {
		t.Fatalf("expected 16 chars, got %d", len(value))
	}
	if _, err := hex.DecodeString(value); err != nil {
		t.Fatalf("expected hex string, got %q", value)
	}
}

func TestRandomHexError(t *testing.T) {
	prevReader := randReader
	randReader = errorReader{}
	t.Cleanup(func() { randReader = prevReader })

	if _, err := randomHex(4); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteAndRemoveEngineState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	state := EngineState{
		Endpoint:   "127.0.0.1:1234",
		PID:        42,
		StartedAt:  "2024-01-01T00:00:00Z",
		AuthToken:  "token",
		Version:    "dev",
		InstanceID: "instance",
	}
	if err := writeEngineState(path, state); err != nil {
		t.Fatalf("writeEngineState: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var decoded EngineState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if decoded.Endpoint != state.Endpoint || decoded.AuthToken != state.AuthToken {
		t.Fatalf("unexpected state: %+v", decoded)
	}
	removeEngineState(path)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected state file removed, got %v", err)
	}
}

func TestWriteEngineStateInvalidDir(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	path := filepath.Join(blocker, "engine.json")
	if err := writeEngineState(path, EngineState{Endpoint: "127.0.0.1:1234"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteEngineStateRemoveError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writeEngineState(path, EngineState{Endpoint: "127.0.0.1:1234"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteEngineStateTempFileError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	tmp := path + ".tmp"
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeEngineState(path, EngineState{Endpoint: "127.0.0.1:1234"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteEngineStateWriteError(t *testing.T) {
	prevWrite := writeFileFn
	writeFileFn = func(string, []byte, os.FileMode) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { writeFileFn = prevWrite })

	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	if err := writeEngineState(path, EngineState{Endpoint: "127.0.0.1:1234"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteEngineStateRenameError(t *testing.T) {
	prevRename := renameFn
	renameFn = func(string, string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() { renameFn = prevRename })

	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	if err := writeEngineState(path, EngineState{Endpoint: "127.0.0.1:1234"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRemoveEngineStateError(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	removeEngineState(nested)
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("expected directory to remain, got %v", err)
	}
}

func TestRunMissingListen(t *testing.T) {
	code, err := run([]string{})
	if code != 2 || err == nil || err.Error() != "missing --listen" {
		t.Fatalf("expected missing --listen, got code=%d err=%v", code, err)
	}
}

func TestRunMissingStatePath(t *testing.T) {
	code, err := run([]string{"--listen=127.0.0.1:0"})
	if code != 2 || err == nil || err.Error() != "missing --write-engine-json" {
		t.Fatalf("expected missing --write-engine-json, got code=%d err=%v", code, err)
	}
}

func TestRunRunDirError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run-dir")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath, "--run-dir=" + path})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "create run dir") {
		t.Fatalf("expected run dir error, got code=%d err=%v", code, err)
	}
}

func TestRunInvalidListen(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=bad", "--write-engine-json=" + statePath})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "listen:") {
		t.Fatalf("expected listen error, got code=%d err=%v", code, err)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	code, err := run([]string{"--nope"})
	if code != 2 || err == nil {
		t.Fatalf("expected parse error, got code=%d err=%v", code, err)
	}
}

func TestRunServeError(t *testing.T) {
	previousServe := serveHTTP
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return errors.New("boom")
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath})
	if err != nil || code != 1 {
		t.Fatalf("expected server error code, got code=%d err=%v", code, err)
	}
}

func TestRunRandomHexError(t *testing.T) {
	prevReader := randReader
	randReader = errorReader{}
	t.Cleanup(func() { randReader = prevReader })

	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "instance id") {
		t.Fatalf("expected instance id error, got code=%d err=%v", code, err)
	}
}

func TestRunAuthTokenError(t *testing.T) {
	prevReader := randReader
	seq := &sequenceReader{failOn: 2}
	randReader = seq
	t.Cleanup(func() { randReader = prevReader })

	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "auth token") {
		t.Fatalf("expected auth token error, got code=%d err=%v", code, err)
	}
}

func TestRunOpenStateDBError(t *testing.T) {
	previousServe := serveHTTP
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	statePath := filepath.Join(blocker, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "open state db") {
		t.Fatalf("expected state db error, got code=%d err=%v", code, err)
	}
}

func TestRunWriteEngineStateError(t *testing.T) {
	previousServe := serveHTTP
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	stateDir := filepath.Join(dir, "engine.json")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + stateDir})
	if code != 1 || err == nil || !strings.Contains(err.Error(), "write engine.json") {
		t.Fatalf("expected write engine.json error, got code=%d err=%v", code, err)
	}
}

func TestRunWithRunDir(t *testing.T) {
	previousServe := serveHTTP
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath, "--run-dir=" + runDir})
	if code != 0 || err != nil {
		t.Fatalf("expected success, got code=%d err=%v", code, err)
	}
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("expected run dir created, got %v", err)
	}
}

func TestRunIdleTimeoutShutdown(t *testing.T) {
	previousServe := serveHTTP
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		return server.Serve(listener)
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath, "--idle-timeout=100ms"})
	if err != nil || code != 0 {
		t.Fatalf("expected success, got code=%d err=%v", code, err)
	}
}

func TestRunSuccess(t *testing.T) {
	previousServe := serveHTTP
	called := false
	serveHTTP = func(server *http.Server, listener net.Listener) error {
		called = true
		_ = listener.Close()
		return http.ErrServerClosed
	}
	t.Cleanup(func() { serveHTTP = previousServe })

	dir := t.TempDir()
	statePath := filepath.Join(dir, "engine.json")
	code, err := run([]string{"--listen=127.0.0.1:0", "--write-engine-json=" + statePath, "--idle-timeout=0"})
	if err != nil || code != 0 {
		t.Fatalf("expected success, got code=%d err=%v", code, err)
	}
	if !called {
		t.Fatalf("expected serveHTTP to be called")
	}
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected engine.json removed, got %v", err)
	}
}

func TestMainUsesExitCode(t *testing.T) {
	prevExit := exitFn
	exitCode := 0
	exitFn = func(code int) {
		exitCode = code
	}
	t.Cleanup(func() { exitFn = prevExit })

	prevArgs := os.Args
	os.Args = []string{"sqlrs-engine"}
	t.Cleanup(func() { os.Args = prevArgs })

	main()
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

type sequenceReader struct {
	calls  int
	failOn int
}

func (r *sequenceReader) Read(p []byte) (int, error) {
	r.calls++
	if r.calls == r.failOn {
		return 0, errors.New("boom")
	}
	for i := range p {
		p[i] = 0x1
	}
	return len(p), nil
}
