package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintPlanUsageShowsRefShape(t *testing.T) {
	var buf bytes.Buffer
	PrintPlanUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "sqlrs plan [--provenance-path <path>] [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] <ref>") {
		t.Fatalf("expected alias-mode ref usage, got %q", out)
	}
	if !strings.Contains(out, "sqlrs plan:psql [--provenance-path <path>] [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--image <image-id>] [--] [psql-args...]") {
		t.Fatalf("expected raw-mode ref usage, got %q", out)
	}
	if !strings.Contains(out, "--provenance-path <path>") {
		t.Fatalf("expected provenance option, got %q", out)
	}
	if !strings.Contains(out, "Relative provenance paths resolve from the command invocation directory.") {
		t.Fatalf("expected provenance path note, got %q", out)
	}
	if !strings.Contains(out, "projected current working directory") {
		t.Fatalf("expected projected cwd note, got %q", out)
	}
}
