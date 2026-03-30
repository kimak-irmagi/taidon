package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/alias"
)

func TestPrintAliasEntries(t *testing.T) {
	var buf bytes.Buffer
	PrintAliasEntries(&buf, []alias.Entry{
		{Class: alias.ClassPrepare, Ref: "chinook", File: "chinook.prep.s9s.yaml", Kind: "psql", Status: "ok"},
		{Class: alias.ClassRun, Ref: "smoke", File: "smoke.run.s9s.yaml", Status: "invalid"},
	})

	out := buf.String()
	for _, want := range []string{"TYPE", "chinook", "smoke", "psql", "INVALID", "-"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestPrintAliasCheck(t *testing.T) {
	var buf bytes.Buffer
	PrintAliasCheck(&buf, alias.CheckReport{
		Checked:      2,
		ValidCount:   1,
		InvalidCount: 1,
		Results: []alias.CheckResult{
			{Type: alias.ClassPrepare, Ref: "chinook", File: "chinook.prep.s9s.yaml", Kind: "psql", Valid: true},
			{Type: alias.ClassRun, Ref: "smoke", File: "smoke.run.s9s.yaml", Valid: false, Error: "referenced path not found: smoke.sql"},
		},
	})

	out := buf.String()
	for _, want := range []string{"STATUS", "VALID", "INVALID", "referenced path not found", "checked=2 valid=1 invalid=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestPrintAliasCreate(t *testing.T) {
	var buf bytes.Buffer
	PrintAliasCreate(&buf, alias.CreateResult{
		File:  "chinook.prep.s9s.yaml",
		Type:  alias.ClassPrepare,
		Ref:   "chinook",
		Kind:  "psql",
		Image: "postgres:17",
	})

	out := buf.String()
	for _, want := range []string{
		"created alias file: chinook.prep.s9s.yaml",
		"type: prepare",
		"ref: chinook",
		"kind: psql",
		"image: postgres:17",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}
