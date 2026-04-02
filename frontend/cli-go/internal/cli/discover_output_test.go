package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/discover"
)

func TestPrintDiscover(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscover(&buf, discover.Report{
		Scanned:     3,
		Prefiltered: 1,
		Validated:   1,
		Suppressed:  0,
		Findings: []discover.Finding{
			{
				Type:          alias.ClassPrepare,
				Kind:          "psql",
				Ref:           "chinook",
				File:          "schema.sql",
				AliasPath:     "chinook.prep.s9s.yaml",
				Reason:        "SQL file",
				CreateCommand: "sqlrs alias create chinook prepare:psql -- -f schema.sql",
				Score:         80,
				Valid:         true,
			},
		},
	})

	out := buf.String()
	if !strings.Contains(out, "scanned=3") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "1. VALID prepare") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "chinook.prep.s9s.yaml") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "sqlrs alias create chinook prepare:psql -- -f schema.sql") {
		t.Fatalf("unexpected output: %q", out)
	}
	if strings.Contains(out, "\t") {
		t.Fatalf("expected block output without tabs, got %q", out)
	}
	if !strings.Contains(out, "   Ref           : chinook") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "   Create command: sqlrs alias create chinook prepare:psql -- -f schema.sql") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPrintDiscoverEmptyReport(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscover(&buf, discover.Report{})

	out := buf.String()
	if !strings.Contains(out, "no advisory alias candidates found") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPrintDiscoverUsage(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscoverUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "sqlrs discover [--aliases]") {
		t.Fatalf("unexpected usage: %q", out)
	}
	if !strings.Contains(out, "read-only") {
		t.Fatalf("unexpected usage: %q", out)
	}
}

func TestPrintUsageMentionsDiscover(t *testing.T) {
	var buf bytes.Buffer
	PrintUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "discover") {
		t.Fatalf("unexpected usage: %q", out)
	}
}

func TestPrintAliasUsageMentionsCreate(t *testing.T) {
	var buf bytes.Buffer
	PrintAliasUsage(&buf)

	out := buf.String()
	if !strings.Contains(out, "sqlrs alias create") {
		t.Fatalf("unexpected usage: %q", out)
	}
	if !strings.Contains(out, "materializes a repo-tracked alias file") {
		t.Fatalf("unexpected usage: %q", out)
	}
}
