package app

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartSpinnerForcedTerminalStopsBeforeShown(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	})

	stop := startSpinner("init", false)
	stop()
	time.Sleep(20 * time.Millisecond)
}

func TestStartSpinnerForcedTerminalShowsSpinner(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = r.Close()
		_ = w.Close()
	})

	stop := startSpinner("init", false)
	time.Sleep(700 * time.Millisecond)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if !strings.Contains(string(data), "init") {
		t.Fatalf("expected spinner output, got %q", string(data))
	}
}

func TestStartCleanupSpinnerForcedTerminalStopsBeforeShown(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	})

	stop := startCleanupSpinner("inst", false)
	stop()
	time.Sleep(20 * time.Millisecond)
}

func TestStartCleanupSpinnerForcedTerminalShowsSpinner(t *testing.T) {
	withIsTerminalWriterStub(t, func(*os.File) bool { return true })

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	})

	stop := startCleanupSpinner("inst", false)
	time.Sleep(700 * time.Millisecond)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(data), "Deleting instance inst") {
		t.Fatalf("expected spinner output, got %q", string(data))
	}
}
