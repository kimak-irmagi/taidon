package cli

import (
	"bytes"
	"strings"
	"testing"

	"sqlrs/cli/internal/client"
)

func TestPrintLsQuietSeparatesSections(t *testing.T) {
	result := LsResult{
		Names: &[]client.NameEntry{
			{
				Name:    "dev",
				ImageID: "image-1",
				StateID: "state-1",
				Status:  "active",
			},
		},
		Instances: &[]client.InstanceEntry{
			{
				InstanceID: "instance-1",
				ImageID:    "image-1",
				StateID:    "state-1",
				CreatedAt:  "2025-01-01T00:00:00Z",
				Status:     "active",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true})

	out := buf.String()
	if strings.Contains(out, "Names") || strings.Contains(out, "Instances") {
		t.Fatalf("unexpected section titles in quiet output: %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatalf("expected blank line between sections, got %q", out)
	}
}
