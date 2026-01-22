package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	PrintUsage(&buf)
	if !strings.Contains(buf.String(), "Usage:") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestPrintCommandUsage(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*bytes.Buffer)
	}{
		{name: "init", fn: func(b *bytes.Buffer) { PrintInitUsage(b) }},
		{name: "ls", fn: func(b *bytes.Buffer) { PrintLsUsage(b) }},
		{name: "plan", fn: func(b *bytes.Buffer) { PrintPlanUsage(b) }},
		{name: "prepare", fn: func(b *bytes.Buffer) { PrintPrepareUsage(b) }},
		{name: "config", fn: func(b *bytes.Buffer) { PrintConfigUsage(b) }},
		{name: "rm", fn: func(b *bytes.Buffer) { PrintRmUsage(b) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.fn(&buf)
			if !strings.Contains(buf.String(), "Usage:") {
				t.Fatalf("unexpected output: %q", buf.String())
			}
		})
	}
}
