package app

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/config"
)

func TestRunPlanMissingImage(t *testing.T) {
	runOpts := cli.PrepareOptions{}
	err := runPlan(&bytes.Buffer{}, io.Discard, runOpts, config.LoadedConfig{}, t.TempDir(), []string{"--", "-c", "select 1"}, "json")
	if err == nil || !strings.Contains(err.Error(), "Missing base image id") {
		t.Fatalf("expected missing image error, got %v", err)
	}
}
