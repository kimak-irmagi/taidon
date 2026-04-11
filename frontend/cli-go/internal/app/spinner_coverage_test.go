package app

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func withSpinnerTimings(t *testing.T, initialDelay, tickInterval time.Duration) {
	t.Helper()
	prevInitialDelay := spinnerInitialDelay
	prevTickInterval := spinnerTickInterval
	spinnerInitialDelay = initialDelay
	spinnerTickInterval = tickInterval
	t.Cleanup(func() {
		spinnerInitialDelay = prevInitialDelay
		spinnerTickInterval = prevTickInterval
	})
}

func TestStartSpinnerForcedTerminalStopsBeforeShown(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })
	withSpinnerTimings(t, 50*time.Millisecond, 10*time.Millisecond)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	}()

	stop := startSpinner("init", false)
	stop()
}

func TestStartSpinnerForcedTerminalShowsSpinner(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })
	withSpinnerTimings(t, 10*time.Millisecond, 10*time.Millisecond)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	}()

	stop := startSpinner("init", false)
	time.Sleep(100 * time.Millisecond)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if !strings.Contains(string(data), "init") {
		t.Fatalf("expected spinner output, got %q", string(data))
	}
	if strings.Contains(string(data), "\x1b[2K") {
		t.Fatalf("expected ANSI-free spinner output, got %q", string(data))
	}
}

func TestStartCleanupSpinnerForcedTerminalStopsBeforeShown(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })
	withSpinnerTimings(t, 50*time.Millisecond, 10*time.Millisecond)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	}()

	stop := startCleanupSpinner("inst", false)
	stop()
}

func TestStartCleanupSpinnerForcedTerminalShowsSpinner(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })
	withSpinnerTimings(t, 10*time.Millisecond, 10*time.Millisecond)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	}()

	stop := startCleanupSpinner("inst", false)
	time.Sleep(100 * time.Millisecond)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(data), "Deleting instance inst") {
		t.Fatalf("expected spinner output, got %q", string(data))
	}
	if strings.Contains(string(data), "\x1b[2K") {
		t.Fatalf("expected ANSI-free spinner output, got %q", string(data))
	}
}
