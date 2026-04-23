package app

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/config"
)

func TestRunCacheHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := runCache(&stdout, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), t.TempDir(), []string{"--help"}, "human")
	if err != nil {
		t.Fatalf("runCache: %v", err)
	}
	if !strings.Contains(stdout.String(), "sqlrs cache explain prepare") {
		t.Fatalf("unexpected help output: %q", stdout.String())
	}
}

func TestRunCacheRejectsUnsupportedWrappedStage(t *testing.T) {
	err := runCache(io.Discard, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), t.TempDir(), []string{"explain", "plan", "chinook"}, "human")
	if err == nil || !strings.Contains(err.Error(), "cache explain only supports prepare stages") {
		t.Fatalf("expected wrapped-stage error, got %v", err)
	}
}

func TestRunCacheRejectsWatchFlags(t *testing.T) {
	err := runCache(io.Discard, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), t.TempDir(), []string{"explain", "prepare:psql", "--watch", "--", "-c", "select 1"}, "human")
	if err == nil || !strings.Contains(err.Error(), "cache explain does not support --watch/--no-watch") {
		t.Fatalf("expected watch rejection, got %v", err)
	}
}

func TestRunCacheRejectsMissingWrappedStage(t *testing.T) {
	err := runCache(io.Discard, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, t.TempDir(), t.TempDir(), []string{"explain"}, "human")
	if err == nil || !strings.Contains(err.Error(), "cache explain requires a wrapped prepare stage") {
		t.Fatalf("expected missing wrapped stage error, got %v", err)
	}
}
