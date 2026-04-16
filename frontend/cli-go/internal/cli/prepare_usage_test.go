package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintPrepareUsageShowsMixedCompositeNote(t *testing.T) {
	var buf bytes.Buffer
	PrintPrepareUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] <ref>") {
		t.Fatalf("expected prepare alias usage, got %q", out)
	}
	if !strings.Contains(out, "sqlrs prepare:psql [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--image <image-id>] [--] [psql-args...]") {
		t.Fatalf("expected raw psql usage with ref flags, got %q", out)
	}
	if !strings.Contains(out, "prepare ... run") {
		t.Fatalf("expected composite note, got %q", out)
	}
	if !strings.Contains(out, "raw and alias stages") {
		t.Fatalf("expected mixed-stage note, got %q", out)
	}
	if !strings.Contains(out, "single-stage only") {
		t.Fatalf("expected bounded ref note, got %q", out)
	}
	if !strings.Contains(out, "--no-watch is rejected with --ref") {
		t.Fatalf("expected ref watch-mode note, got %q", out)
	}
}
