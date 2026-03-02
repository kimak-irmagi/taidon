package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWatchUsageIncludesSyntax(t *testing.T) {
	var buf bytes.Buffer
	PrintWatchUsage(&buf)
	out := buf.String()
	if !strings.Contains(out, "sqlrs watch <job-id>") {
		t.Fatalf("expected watch syntax, got:\n%s", out)
	}
}
