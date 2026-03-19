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
	if !strings.Contains(out, "sqlrs prepare [--watch|--no-watch] <ref>") {
		t.Fatalf("expected prepare alias usage, got %q", out)
	}
	if !strings.Contains(out, "prepare ... run") {
		t.Fatalf("expected composite note, got %q", out)
	}
	if !strings.Contains(out, "raw and alias stages") {
		t.Fatalf("expected mixed-stage note, got %q", out)
	}
}
