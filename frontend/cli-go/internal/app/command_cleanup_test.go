package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
)

func TestCleanupPreparedInstanceSuccess(t *testing.T) {
	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	var started string
	var stopped bool
	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() {
		started = instanceID
		return func() { stopped = true }
	}
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		return client.DeleteResult{Outcome: "deleted"}, http.StatusOK, nil
	}

	var stderr bytes.Buffer
	cleanupPreparedInstance(context.Background(), &stderr, cli.RunOptions{}, "inst-1", false)

	if started != "inst-1" {
		t.Fatalf("spinner started for %q, want inst-1", started)
	}
	if !stopped {
		t.Fatalf("expected spinner to stop")
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestCleanupPreparedInstanceBlocked(t *testing.T) {
	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() { return func() {} }
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		count := 1
		return client.DeleteResult{
			Outcome: "blocked",
			Root: client.DeleteNode{
				Blocked:     "runtime",
				Connections: &count,
			},
		}, http.StatusConflict, nil
	}

	var stderr bytes.Buffer
	cleanupPreparedInstance(context.Background(), &stderr, cli.RunOptions{}, "inst-1", true)

	out := stderr.String()
	if !strings.Contains(out, "cleanup blocked for instance inst-1") {
		t.Fatalf("unexpected stderr: %q", out)
	}
	if !strings.Contains(out, "blocked=runtime") || !strings.Contains(out, "connections=1") {
		t.Fatalf("expected formatted cleanup details, got %q", out)
	}
}

func TestCleanupPreparedInstanceErrorVerbose(t *testing.T) {
	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() { return func() {} }
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		return client.DeleteResult{}, http.StatusInternalServerError, errors.New("boom")
	}

	var stderr bytes.Buffer
	cleanupPreparedInstance(context.Background(), &stderr, cli.RunOptions{}, "inst-1", true)

	out := stderr.String()
	if !strings.Contains(out, "cleanup failed for instance inst-1: boom") {
		t.Fatalf("unexpected stderr: %q", out)
	}
}

func TestCleanupPreparedInstanceErrorNonVerbose(t *testing.T) {
	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() { return func() {} }
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		return client.DeleteResult{}, http.StatusInternalServerError, errors.New("boom")
	}

	var stderr bytes.Buffer
	cleanupPreparedInstance(context.Background(), &stderr, cli.RunOptions{}, "inst-1", false)

	out := stderr.String()
	if !strings.Contains(out, "cleanup failed: boom") {
		t.Fatalf("unexpected stderr: %q", out)
	}
	if strings.Contains(out, "inst-1") {
		t.Fatalf("expected non-verbose output without instance id, got %q", out)
	}
}

func TestCleanupPreparedInstanceBlankInstanceSkipsWork(t *testing.T) {
	prevDelete := deleteInstanceDetailedFn
	prevSpinner := startCleanupSpinnerFn
	t.Cleanup(func() {
		deleteInstanceDetailedFn = prevDelete
		startCleanupSpinnerFn = prevSpinner
	})

	called := false
	startCleanupSpinnerFn = func(instanceID string, verbose bool) func() {
		called = true
		return func() {}
	}
	deleteInstanceDetailedFn = func(ctx context.Context, opts cli.RunOptions, instanceID string) (client.DeleteResult, int, error) {
		t.Fatalf("deleteInstanceDetailedFn should not be called")
		return client.DeleteResult{}, 0, nil
	}

	cleanupPreparedInstance(context.Background(), &bytes.Buffer{}, cli.RunOptions{}, "  ", false)
	if called {
		t.Fatalf("expected blank instance to skip spinner")
	}
}
