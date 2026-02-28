package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrepareUsageIncludesLiquibase(t *testing.T) {
	var buf bytes.Buffer
	PrintPrepareUsage(&buf)
	out := buf.String()
	if !strings.Contains(out, "prepare:psql") {
		t.Fatalf("expected prepare:psql in usage, got:\n%s", out)
	}
	if !strings.Contains(out, "prepare:lb") {
		t.Fatalf("expected prepare:lb in usage, got:\n%s", out)
	}
	if !strings.Contains(out, "--no-watch") {
		t.Fatalf("expected --no-watch in usage, got:\n%s", out)
	}
}
