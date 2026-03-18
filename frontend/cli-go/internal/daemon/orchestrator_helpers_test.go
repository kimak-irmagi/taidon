package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFormatEngineExit(t *testing.T) {
	if err := formatEngineExit(nil); err == nil || !strings.Contains(err.Error(), "exit code 0") {
		t.Fatalf("expected exit code 0 message, got %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessExit", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_EXIT_CODE=7")
	runErr := cmd.Run()
	if runErr == nil {
		t.Fatalf("expected helper process error")
	}
	if err := formatEngineExit(runErr); err == nil || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("expected exit code 7 message, got %v", err)
	}

	if err := formatEngineExit(errors.New("boom")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected generic error message, got %v", err)
	}
}

func TestHelperProcessExit(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT_CODE"))
	os.Exit(code)
}

func TestHelperProcessCommandOutput(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "command-output" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR"))
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT_CODE"))
	os.Exit(code)
}

func TestRunCommandWithSplitOutputIgnoresStderrOnSuccess(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "command-output")
	t.Setenv("HELPER_STDOUT", "active\n")
	t.Setenv("HELPER_STDERR", "wsl: localhost proxy warning\n")
	t.Setenv("HELPER_EXIT_CODE", "0")

	out, err := runCommandWithSplitOutput(context.Background(), os.Args[0], "-test.run=TestHelperProcessCommandOutput", "--")
	if err != nil {
		t.Fatalf("runCommandWithSplitOutput: %v", err)
	}
	if out != "active\n" {
		t.Fatalf("expected clean stdout, got %q", out)
	}
}

func TestRunCommandWithSplitOutputKeepsStdoutOnFailure(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "command-output")
	t.Setenv("HELPER_STDOUT", "inactive\n")
	t.Setenv("HELPER_STDERR", "wsl: localhost proxy warning\n")
	t.Setenv("HELPER_EXIT_CODE", "3")

	out, err := runCommandWithSplitOutput(context.Background(), os.Args[0], "-test.run=TestHelperProcessCommandOutput", "--")
	if err == nil {
		t.Fatalf("expected command error")
	}
	if out != "inactive\n" {
		t.Fatalf("expected stdout to be preserved, got %q", out)
	}
	if !strings.Contains(err.Error(), "localhost proxy warning") {
		t.Fatalf("expected stderr detail in error, got %v", err)
	}
}

func TestEnsureWSLMountUnitActive(t *testing.T) {
	old := runWSLCommandFn
	t.Cleanup(func() { runWSLCommandFn = old })

	step := 0
	runWSLCommandFn = func(ctx context.Context, distro string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "is-active":
			step++
			if step == 1 {
				return "inactive\n", nil
			}
			return "active\n", nil
		case len(args) >= 2 && args[0] == "systemctl" && args[1] == "start":
			return "", nil
		case len(args) >= 1 && args[0] == "journalctl":
			return "logs", nil
		default:
			return "", nil
		}
	}

	if err := ensureWSLMountUnitActive(context.Background(), "ubuntu", "unit"); err != nil {
		t.Fatalf("ensureWSLMountUnitActive: %v", err)
	}
}

func TestAttachVHDXToWSL(t *testing.T) {
	if err := attachVHDXToWSL(context.Background(), "", false); err == nil {
		t.Fatalf("expected empty path error")
	}

	old := runHostCommandFn
	t.Cleanup(func() { runHostCommandFn = old })

	runHostCommandFn = func(ctx context.Context, args ...string) (string, error) {
		return "boom", errors.New("failed")
	}
	err := attachVHDXToWSL(context.Background(), "C:\\path\\file.vhdx", false)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected attach error with output, got %v", err)
	}
}

func TestStartLogTail(t *testing.T) {
	if _, err := startLogTail("missing.log", nil); err == nil {
		t.Fatalf("expected error for nil output")
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "engine.log")
	if err := os.WriteFile(logPath, []byte(""), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var buf bytes.Buffer
	tail, err := startLogTail(logPath, &buf)
	if err != nil {
		t.Fatalf("startLogTail: %v", err)
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("write log: %v", err)
	}
	f.Close()
	time.Sleep(60 * time.Millisecond)
	tail.Stop()
}
