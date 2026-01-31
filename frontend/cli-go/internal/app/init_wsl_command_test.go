package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWSLUnavailable(t *testing.T) {
	opts := wslInitOptions{Require: false}
	result, err := wslUnavailable(opts, "no wsl")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.UseWSL {
		t.Fatalf("expected UseWSL false")
	}
	if result.Warning != "no wsl" {
		t.Fatalf("expected warning, got %q", result.Warning)
	}

	_, err = wslUnavailable(wslInitOptions{Require: true}, "hard fail")
	if err == nil || err.Error() != "hard fail" {
		t.Fatalf("expected hard fail, got %v", err)
	}
}

func TestRunWSLCommandSuccess(t *testing.T) {
	withFakeWSL(t)
	t.Setenv("WSL_STDOUT", "ok")
	out, err := runWSLCommand(context.Background(), "Ubuntu", false, "test", "true")
	if err != nil {
		t.Fatalf("runWSLCommand: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("expected ok, got %q", out)
	}
}

func TestRunWSLCommandWithInput(t *testing.T) {
	withFakeWSL(t)
	t.Setenv("WSL_ECHO_STDIN", "1")
	out, err := runWSLCommandWithInput(context.Background(), "Ubuntu", false, "test", "payload", "true")
	if err != nil {
		t.Fatalf("runWSLCommandWithInput: %v", err)
	}
	if out != "payload" {
		t.Fatalf("expected payload, got %q", out)
	}
}

func TestRunWSLCommandAllowFailure(t *testing.T) {
	withFakeWSL(t)
	t.Setenv("WSL_EXIT", "2")
	t.Setenv("WSL_STDERR", "boom")
	_, err := runWSLCommandAllowFailure(context.Background(), "Ubuntu", false, "test", "true")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected stderr error, got %v", err)
	}
}

func TestRunHostCommandSuccess(t *testing.T) {
	ctx := context.Background()
	var cmd string
	var args []string
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/c", "echo", "ok"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo ok"}
	}
	out, err := runHostCommand(ctx, false, "host", cmd, args...)
	if err != nil {
		t.Fatalf("runHostCommand: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("expected ok, got %q", out)
	}
}

func TestIsElevatedUsesHostCommand(t *testing.T) {
	prev := runHostCommandFn
	runHostCommandFn = func(context.Context, bool, string, string, ...string) (string, error) {
		return "True", nil
	}
	t.Cleanup(func() { runHostCommandFn = prev })
	ok, err := isElevated(false)
	if err != nil || !ok {
		t.Fatalf("expected elevated true, got %v err=%v", ok, err)
	}

	runHostCommandFn = func(context.Context, bool, string, string, ...string) (string, error) {
		return "False", nil
	}
	ok, err = isElevated(false)
	if err != nil || ok {
		t.Fatalf("expected elevated false, got %v err=%v", ok, err)
	}
}

func TestStartSpinnerAndClearLine(t *testing.T) {
	stop := startSpinner("noop", true)
	stop()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prev := os.Stderr
	os.Stderr = w
	clearLine()
	_ = w.Close()
	os.Stderr = prev
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "\r") {
		t.Fatalf("expected clear line output, got %q", buf.String())
	}
}

func withFakeWSL(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	var wslPath string
	if runtime.GOOS == "windows" {
		wslPath = filepath.Join(dir, "wsl.exe")
		writeFakeWSLGo(t, dir, wslPath)
	} else {
		wslPath = filepath.Join(dir, "wsl.exe")
		script := "#!/bin/sh\n" +
			"if [ -n \"$WSL_STDERR\" ]; then printf \"%s\" \"$WSL_STDERR\" 1>&2; fi\n" +
			"if [ -n \"$WSL_ECHO_STDIN\" ]; then cat; fi\n" +
			"if [ -n \"$WSL_STDOUT\" ]; then printf \"%s\" \"$WSL_STDOUT\"; fi\n" +
			"if [ -n \"$WSL_EXIT\" ]; then exit \"$WSL_EXIT\"; fi\n" +
			"exit 0\n"
		if err := os.WriteFile(wslPath, []byte(script), 0o700); err != nil {
			t.Fatalf("write wsl script: %v", err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := os.Stat(wslPath); err != nil {
		t.Fatalf("fake wsl not created: %v", err)
	}
}

func writeFakeWSLGo(t *testing.T, dir, outPath string) {
	t.Helper()
	src := `package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

func main() {
	if v := os.Getenv("WSL_STDERR"); v != "" {
		fmt.Fprint(os.Stderr, v)
	}
	if v := os.Getenv("WSL_ECHO_STDIN"); v != "" {
		data, _ := io.ReadAll(os.Stdin)
		fmt.Fprint(os.Stdout, string(data))
	}
	if v := os.Getenv("WSL_STDOUT"); v != "" {
		fmt.Fprint(os.Stdout, v)
	}
	if v := os.Getenv("WSL_EXIT"); v != "" {
		code, _ := strconv.Atoi(v)
		os.Exit(code)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module wslstub\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte(src), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", outPath, mainPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake wsl: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("built wsl.exe missing: %v", err)
	}
}
