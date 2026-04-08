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
		SelectedAnalyzers: []string{discover.AnalyzerAliases, discover.AnalyzerGitignore},
		Scanned:           3,
		Prefiltered:       1,
		Validated:         1,
		Suppressed:        0,
		Findings: []discover.Finding{
			{
				Analyzer:      discover.AnalyzerAliases,
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
			{
				Analyzer: discover.AnalyzerGitignore,
				Target:   ".gitignore",
				Action:   "add missing ignore entries",
				Reason:   "local sqlrs workspace state should stay out of version control",
				SuggestedEntries: []string{
					".sqlrs/",
				},
				FollowUpCommand: &discover.FollowUpCommand{
					ShellFamily: discover.ShellFamilyPOSIX,
					Command:     "grep -qxF '.sqlrs/' .gitignore 2>/dev/null || printf '%s\\n' '.sqlrs/' >> .gitignore",
				},
				Valid: true,
			},
		},
	})

	out := buf.String()
	for _, want := range []string{
		"selected_analyzers=aliases,gitignore",
		"[aliases]",
		"1. VALID prepare",
		"chinook.prep.s9s.yaml",
		"sqlrs alias create chinook prepare:psql -- -f schema.sql",
		"   Ref           : chinook",
		"   Create command: sqlrs alias create chinook prepare:psql -- -f schema.sql",
		"[gitignore]",
		"2. ADVISORY gitignore",
		"   Target        : .gitignore",
		"   Entries       : .sqlrs/",
		"   Shell         : posix",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected output, missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "\t") {
		t.Fatalf("expected block output without tabs, got %q", out)
	}
}

func TestPrintDiscoverFallbackOrderingAndMultilineFields(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscover(&buf, discover.Report{
		Scanned: 1,
		Findings: []discover.Finding{
			{
				Analyzer:    discover.AnalyzerVSCode,
				Target:      ".vscode/settings.json",
				Action:      "add missing VS Code yaml schema guidance",
				JSONPayload: "{\n  \"yaml.schemas\": {}\n}",
				Error:       "invalid json payload",
				Valid:       false,
			},
			{
				Analyzer: discover.AnalyzerAliases,
				Type:     alias.ClassPrepare,
				Kind:     "psql",
				Ref:      "demo",
				Error:    "validation failed",
				Valid:    false,
				FollowUpCommand: &discover.FollowUpCommand{
					Command: "sqlrs alias create demo prepare:psql -- -f demo.sql",
				},
			},
		},
	})

	out := buf.String()
	for _, want := range []string{
		"[aliases]",
		"1. INVALID prepare",
		"   Error         : validation failed",
		"   Create command: sqlrs alias create demo prepare:psql -- -f demo.sql",
		"[vscode]",
		"2. ADVISORY vscode",
		"   Payload       : {",
		"                  \"yaml.schemas\": {}",
		"   Error         : invalid json payload",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected output, missing %q: %q", want, out)
		}
	}
}

func TestPrintAliasDiscoverFindingFallbacks(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscover(&buf, discover.Report{
		SelectedAnalyzers: []string{discover.AnalyzerAliases},
		Findings: []discover.Finding{{
			Analyzer: discover.AnalyzerAliases,
			Type:     alias.ClassPrepare,
			Valid:    false,
		}},
	})

	out := buf.String()
	for _, want := range []string{
		"1. INVALID prepare",
		"   Error         : -",
		"   Create command: -",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected output, missing %q: %q", want, out)
		}
	}
}

func TestPrintDiscoverEmptyReport(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscover(&buf, discover.Report{})

	out := buf.String()
	if !strings.Contains(out, "no advisory discover findings") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPrintDiscoverUsage(t *testing.T) {
	var buf bytes.Buffer
	PrintDiscoverUsage(&buf)

	out := buf.String()
	for _, want := range []string{
		"sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]",
		"--gitignore",
		"--vscode",
		"read-only",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unexpected usage, missing %q: %q", want, out)
		}
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
