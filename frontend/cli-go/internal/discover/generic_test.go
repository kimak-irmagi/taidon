package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSelectedAnalyzers(t *testing.T) {
	got, err := NormalizeSelectedAnalyzers(nil)
	if err != nil {
		t.Fatalf("NormalizeSelectedAnalyzers default: %v", err)
	}
	want := []string{AnalyzerAliases, AnalyzerGitignore, AnalyzerVSCode, AnalyzerPrepareShaping}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default analyzers = %v, want %v", got, want)
	}

	got, err = NormalizeSelectedAnalyzers([]string{AnalyzerVSCode, AnalyzerAliases, AnalyzerVSCode, AnalyzerGitignore})
	if err != nil {
		t.Fatalf("NormalizeSelectedAnalyzers subset: %v", err)
	}
	want = []string{AnalyzerAliases, AnalyzerGitignore, AnalyzerVSCode}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("subset analyzers = %v, want %v", got, want)
	}

	if _, err := NormalizeSelectedAnalyzers([]string{"nope"}); err == nil {
		t.Fatal("expected unknown analyzer error")
	}
}

func TestAnalyzeMergesReportsAndKeepsOtherAnalyzerFindingsOnFailure(t *testing.T) {
	prev := analyzerRegistry
	analyzerRegistry = map[string]analyzerRunner{
		AnalyzerAliases: func(opts Options) (Report, error) {
			return Report{
				SelectedAnalyzers: []string{AnalyzerAliases},
				Findings: []Finding{{
					Analyzer: AnalyzerAliases,
					Type:     "prepare",
					Kind:     "psql",
					Ref:      "schema",
					Valid:    true,
				}},
			}, nil
		},
		AnalyzerGitignore: func(opts Options) (Report, error) {
			return Report{}, os.ErrInvalid
		},
	}
	t.Cleanup(func() { analyzerRegistry = prev })

	report, err := Analyze(Options{
		WorkspaceRoot:     t.TempDir(),
		SelectedAnalyzers: []string{AnalyzerGitignore, AnalyzerAliases},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := strings.Join(report.SelectedAnalyzers, ","); got != "aliases,gitignore" {
		t.Fatalf("unexpected selected analyzers: %q", got)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected two findings, got %+v", report)
	}
	if report.Findings[0].Analyzer != AnalyzerAliases {
		t.Fatalf("expected aliases finding first, got %+v", report.Findings)
	}
	if report.Findings[1].Analyzer != AnalyzerGitignore || report.Findings[1].Error == "" {
		t.Fatalf("expected gitignore error finding, got %+v", report.Findings[1])
	}
}

func TestAnalyzeGitignoreFindsRootAndNestedTargets(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir .sqlrs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "nested"), 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "nested", "coverage-current"), []byte("snapshot"), 0o600); err != nil {
		t.Fatalf("write coverage-current: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".gitignore"), []byte(""), 0o600); err != nil {
		t.Fatalf("write root .gitignore: %v", err)
	}

	report, err := AnalyzeGitignore(Options{WorkspaceRoot: workspace, ShellFamily: ShellFamilyPOSIX})
	if err != nil {
		t.Fatalf("AnalyzeGitignore: %v", err)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("expected two gitignore findings, got %+v", report)
	}

	targets := make(map[string]Finding, len(report.Findings))
	for _, finding := range report.Findings {
		targets[finding.Target] = finding
	}
	rootFinding, ok := targets[".gitignore"]
	if !ok || len(rootFinding.SuggestedEntries) != 1 || rootFinding.SuggestedEntries[0] != ".sqlrs/" {
		t.Fatalf("missing root .gitignore finding: %+v", report.Findings)
	}
	nestedFinding, ok := targets[filepath.ToSlash(filepath.Join("nested", ".gitignore"))]
	if !ok || len(nestedFinding.SuggestedEntries) != 1 || nestedFinding.SuggestedEntries[0] != "coverage-current" {
		t.Fatalf("missing nested .gitignore finding: %+v", report.Findings)
	}
	if nestedFinding.FollowUpCommand == nil || !strings.Contains(nestedFinding.FollowUpCommand.Command, "coverage-current") {
		t.Fatalf("expected follow-up command for nested gitignore finding: %+v", nestedFinding)
	}
}

func TestAnalyzeGitignoreSkipsCoveredEntries(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir .sqlrs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".gitignore"), []byte(".sqlrs/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	report, err := AnalyzeGitignore(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzeGitignore: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for covered .sqlrs entry, got %+v", report)
	}
}

func TestAnalyzeVSCodeCreatesOrMergesSettings(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, ".vscode", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir .vscode: %v", err)
	}
	if err := os.WriteFile(target, []byte("{\n  \"editor.tabSize\": 2\n}\n"), 0o600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	report, err := AnalyzeVSCode(Options{WorkspaceRoot: workspace, ShellFamily: ShellFamilyPOSIX})
	if err != nil {
		t.Fatalf("AnalyzeVSCode: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one vscode finding, got %+v", report)
	}
	finding := report.Findings[0]
	if finding.FollowUpCommand == nil || finding.JSONPayload == "" {
		t.Fatalf("expected follow-up command and payload, got %+v", finding)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(finding.JSONPayload), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["editor.tabSize"] != float64(2) {
		t.Fatalf("expected unrelated setting to survive merge: %+v", payload)
	}
	yamlSchemas, ok := payload["yaml.schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected yaml.schemas map: %+v", payload)
	}
	if _, ok := yamlSchemas["./.vscode/sqlrs-workspace-config.schema.json"]; !ok {
		t.Fatalf("expected sqlrs schema mapping: %+v", yamlSchemas)
	}
}

func TestAnalyzePrepareShapingFindsMixedStableAndVolatileInputs(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "db"), 0o700); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "db", "schema.sql"), []byte("create table users(id int);\n"), 0o600); err != nil {
		t.Fatalf("write schema.sql: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "db", "seed.sql"), []byte("insert into users values (1);\n"), 0o600); err != nil {
		t.Fatalf("write seed.sql: %v", err)
	}

	report, err := AnalyzePrepareShaping(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzePrepareShaping: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected one shaping finding, got %+v", report)
	}
	if report.Findings[0].Analyzer != AnalyzerPrepareShaping || !strings.Contains(report.Findings[0].Action, "split stable base") {
		t.Fatalf("unexpected shaping finding: %+v", report.Findings[0])
	}
}

func TestRenderFollowUpCommandsUsesShellFamily(t *testing.T) {
	ps := renderGitignoreCommand(ShellFamilyPowerShell, ".gitignore", []string{".sqlrs/"})
	if !strings.Contains(ps, "Add-Content") {
		t.Fatalf("expected PowerShell append command, got %q", ps)
	}
	posix := renderGitignoreCommand(ShellFamilyPOSIX, ".gitignore", []string{".sqlrs/"})
	if !strings.Contains(posix, "grep -qxF") {
		t.Fatalf("expected POSIX append command, got %q", posix)
	}

	jsonPS := renderJSONWriteCommand(ShellFamilyPowerShell, filepath.ToSlash(filepath.Join(".vscode", "settings.json")), []byte("{\n}\n"))
	if !strings.Contains(jsonPS, "Set-Content") {
		t.Fatalf("expected PowerShell json command, got %q", jsonPS)
	}
	jsonPOSIX := renderJSONWriteCommand(ShellFamilyPOSIX, filepath.ToSlash(filepath.Join(".vscode", "settings.json")), []byte("{\n}\n"))
	if !strings.Contains(jsonPOSIX, "cat <<'EOF'") {
		t.Fatalf("expected POSIX json command, got %q", jsonPOSIX)
	}
}
