package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintRunUsageShowsAliasAndRawModes(t *testing.T) {
	var buf bytes.Buffer
	PrintRunUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "sqlrs run <run-ref> --instance <id|name>") {
		t.Fatalf("expected alias-mode usage, got %q", out)
	}
	if !strings.Contains(out, "sqlrs run[:kind] [--instance <id|name>] [-- <command> ] [args...]") {
		t.Fatalf("expected raw-mode usage, got %q", out)
	}
	if !strings.Contains(out, "prepare ... run") {
		t.Fatalf("expected composite note, got %q", out)
	}
}
