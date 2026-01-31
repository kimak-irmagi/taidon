package app

import (
	"bytes"
	"strings"
	"testing"

	"sqlrs/cli/internal/client"
)

func TestFormatCleanupResultEmpty(t *testing.T) {
	if got := formatCleanupResult(client.DeleteResult{}); got != "blocked" {
		t.Fatalf("expected blocked, got %s", got)
	}
}

func TestFormatCleanupResultFields(t *testing.T) {
	count := 3
	result := client.DeleteResult{
		Outcome: "blocked",
		Root: client.DeleteNode{
			Blocked:     "runtime",
			Connections: &count,
		},
	}
	got := formatCleanupResult(result)
	want := "outcome=blocked, blocked=runtime, connections=3"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestClearLineOut(t *testing.T) {
	var buf bytes.Buffer
	clearLineOut(&buf, 3)
	got := buf.String()
	if !strings.HasPrefix(got, "\r") || !strings.HasSuffix(got, "\r") {
		t.Fatalf("expected carriage returns, got %q", got)
	}
	if !strings.Contains(got, "   ") {
		t.Fatalf("expected spaces, got %q", got)
	}
}
