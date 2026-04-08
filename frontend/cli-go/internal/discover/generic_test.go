package discover

import (
	"encoding/json"
	"errors"
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

	got, err = NormalizeSelectedAnalyzers([]string{" ", AnalyzerAliases})
	if err != nil {
		t.Fatalf("NormalizeSelectedAnalyzers whitespace: %v", err)
	}
	if strings.Join(got, ",") != AnalyzerAliases {
		t.Fatalf("whitespace analyzers = %v, want %v", got, []string{AnalyzerAliases})
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

func TestAnalyzeNormalizesShellFamilyAndSummaries(t *testing.T) {
	prev := analyzerRegistry
	analyzerRegistry = map[string]analyzerRunner{
		AnalyzerAliases: func(opts Options) (Report, error) {
			if opts.ShellFamily != normalizedShellFamily("unsupported") {
				t.Fatalf("expected normalized shell family, got %q", opts.ShellFamily)
			}
			return Report{
				Findings: []Finding{{
					Analyzer: AnalyzerAliases,
					Valid:    true,
				}},
			}, nil
		},
	}
	t.Cleanup(func() { analyzerRegistry = prev })

	report, err := Analyze(Options{
		WorkspaceRoot:     t.TempDir(),
		SelectedAnalyzers: []string{AnalyzerAliases},
		ShellFamily:       "unsupported",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(report.Summaries) != 1 || report.Summaries[0].Analyzer != AnalyzerAliases || report.Summaries[0].Findings != 1 {
		t.Fatalf("unexpected summaries: %+v", report.Summaries)
	}
}

func TestAnalyzeSkipsNilAnalyzerRunner(t *testing.T) {
	prev := analyzerRegistry
	analyzerRegistry = map[string]analyzerRunner{
		AnalyzerAliases: nil,
	}
	t.Cleanup(func() { analyzerRegistry = prev })

	report, err := Analyze(Options{
		WorkspaceRoot:     t.TempDir(),
		SelectedAnalyzers: []string{AnalyzerAliases},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(report.Findings) != 0 || len(report.Summaries) != 0 {
		t.Fatalf("expected skipped nil runner, got %+v", report)
	}
}

func TestAnalyzePropagatesSelectionNormalizationError(t *testing.T) {
	if _, err := Analyze(Options{
		WorkspaceRoot:     t.TempDir(),
		SelectedAnalyzers: []string{"unknown"},
	}); err == nil {
		t.Fatal("expected selection normalization error")
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

func TestAnalyzeGitignoreRejectsMissingWorkspaceAndReportsReadErrors(t *testing.T) {
	if _, err := AnalyzeGitignore(Options{}); err == nil {
		t.Fatal("expected missing workspace root error")
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".sqlrs"), 0o700); err != nil {
		t.Fatalf("mkdir .sqlrs: %v", err)
	}
	if err := os.Mkdir(filepath.Join(workspace, ".gitignore"), 0o700); err != nil {
		t.Fatalf("mkdir .gitignore dir: %v", err)
	}

	report, err := AnalyzeGitignore(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzeGitignore: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Error == "" || report.Findings[0].Valid {
		t.Fatalf("expected invalid gitignore finding, got %+v", report.Findings)
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

func TestAnalyzeVSCodeRejectsMissingWorkspaceAndHandlesNoChangeOrInvalidJSON(t *testing.T) {
	if _, err := AnalyzeVSCode(Options{}); err == nil {
		t.Fatal("expected missing workspace root error")
	}

	workspace := t.TempDir()
	target := filepath.Join(workspace, ".vscode", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("mkdir .vscode: %v", err)
	}
	if err := os.WriteFile(target, []byte("{\n  \"yaml.schemas\": {\n    \"./.vscode/sqlrs-workspace-config.schema.json\": [\"**/.sqlrs/config.yaml\"]\n  }\n}\n"), 0o600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	report, err := AnalyzeVSCode(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzeVSCode no-op: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no-op vscode report, got %+v", report)
	}

	if err := os.WriteFile(target, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write invalid settings.json: %v", err)
	}
	report, err = AnalyzeVSCode(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzeVSCode invalid json: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Error == "" || report.Findings[0].Valid {
		t.Fatalf("expected invalid vscode finding, got %+v", report.Findings)
	}

	if data, err := osReadFile(filepath.Join(workspace, "missing.json")); err != nil || data != nil {
		t.Fatalf("expected osReadFile missing-file fallback, got %q, %v", string(data), err)
	}
}

func TestMergeVSCodeSettingsFileAndSchemaMappingBranches(t *testing.T) {
	prevReadFile := osReadFile
	osReadFile = func(string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	if _, _, err := mergeVSCodeSettingsFile("ignored"); err == nil {
		t.Fatal("expected read error")
	}
	osReadFile = prevReadFile

	emptyPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty settings file: %v", err)
	}
	if merged, changed, err := mergeVSCodeSettingsFile(emptyPath); err != nil || !changed || !strings.Contains(string(merged), "yaml.schemas") {
		t.Fatalf("expected empty settings file to become merged payload, got changed=%v err=%v payload=%q", changed, err, string(merged))
	}

	root := map[string]any{}
	if !ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected schema mapping to be added")
	}
	if ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected second call to be no-op")
	}

	root = map[string]any{
		"yaml.schemas": map[string]any{
			"./.vscode/sqlrs-workspace-config.schema.json": []any{"already-present"},
		},
	}
	if !ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected []any branch to append glob")
	}

	root = map[string]any{
		"yaml.schemas": map[string]any{
			"./.vscode/sqlrs-workspace-config.schema.json": []string{"already-present"},
		},
	}
	if !ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected []string branch to append missing glob")
	}

	root = map[string]any{
		"yaml.schemas": map[string]any{
			"./.vscode/sqlrs-workspace-config.schema.json": []string{"**/.sqlrs/config.yaml"},
		},
	}
	if ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected []string branch with existing glob to stay unchanged")
	}

	root = map[string]any{
		"yaml.schemas": map[string]any{
			"./.vscode/sqlrs-workspace-config.schema.json": "wrong-shape",
		},
	}
	if !ensureVSCodeSchemaMapping(root) {
		t.Fatal("expected fallback branch to replace wrong shape")
	}

	if ensureVSCodeSchemaMapping(nil) {
		t.Fatal("expected nil root to stay unchanged")
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

func TestAnalyzePrepareShapingRejectsMissingWorkspaceAndSkipsUnrelatedFiles(t *testing.T) {
	if _, err := AnalyzePrepareShaping(Options{}); err == nil {
		t.Fatal("expected missing workspace root error")
	}

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o700); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "docs", "README.md"), []byte("notes"), 0o600); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	report, err := AnalyzePrepareShaping(Options{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("AnalyzePrepareShaping: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no prepare-shaping findings, got %+v", report)
	}
}

func TestRenderFollowUpCommandsUsesShellFamily(t *testing.T) {
	if got := renderGitignoreCommand(ShellFamilyPOSIX, ".gitignore", nil); got != "" {
		t.Fatalf("expected empty gitignore command, got %q", got)
	}
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
