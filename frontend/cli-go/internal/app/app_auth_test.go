package app

import (
	"io"
	"os"
	"strings"
	"testing"

	"sqlrs/cli/internal/config"
)

func TestResolveAuthTokenUsesEnv(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "env-token")
	got := resolveAuthToken(config.AuthConfig{TokenEnv: "SQLRS_TOKEN", Token: "fallback"})
	if got != "env-token" {
		t.Fatalf("expected env token, got %q", got)
	}
}

func TestResolveAuthTokenFallsBackToToken(t *testing.T) {
	t.Setenv("SQLRS_TOKEN", "")
	got := resolveAuthToken(config.AuthConfig{TokenEnv: "SQLRS_TOKEN", Token: " token "})
	if got != "token" {
		t.Fatalf("expected fallback token, got %q", got)
	}
}

func TestStartCleanupSpinnerVerboseWritesLabel(t *testing.T) {
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

	stop := startCleanupSpinner("inst-1", true)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(data), "Deleting instance inst-1") {
		t.Fatalf("expected label, got %q", string(data))
	}
}

func TestStartCleanupSpinnerNonTerminalWritesLabel(t *testing.T) {
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

	stop := startCleanupSpinner("inst-2", false)
	stop()
	_ = w.Close()
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(data), "Deleting instance inst-2") {
		t.Fatalf("expected label, got %q", string(data))
	}
}
