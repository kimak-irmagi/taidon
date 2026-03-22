package alias

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli/runkind"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
	"gopkg.in/yaml.v3"
)

// CheckScan validates every selected alias discovered by scan mode.
func CheckScan(opts ScanOptions) (CheckReport, error) {
	entries, err := Scan(opts)
	if err != nil {
		return CheckReport{}, err
	}
	report := CheckReport{
		Checked: len(entries),
		Results: make([]CheckResult, 0, len(entries)),
	}
	for _, entry := range entries {
		result, err := CheckTarget(Target{
			Class: entry.Class,
			Ref:   entry.Ref,
			File:  entry.File,
			Path:  entry.Path,
		}, opts.WorkspaceRoot)
		if err != nil {
			return CheckReport{}, err
		}
		if result.Valid {
			report.ValidCount++
		} else {
			report.InvalidCount++
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

// CheckTarget performs static alias validation without runtime work.
func CheckTarget(target Target, workspaceRoot string) (CheckResult, error) {
	result := CheckResult{
		Type: target.Class,
		Ref:  target.Ref,
		File: target.File,
		Path: target.Path,
	}

	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return result, fmt.Errorf("workspace root is required for alias validation")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)
	if !isWithin(canonicalizeBoundaryPath(workspaceRoot), canonicalizeBoundaryPath(target.Path)) {
		return result, fmt.Errorf("alias path must stay within workspace root: %s", target.Path)
	}

	var (
		kind   string
		issues []Issue
	)
	switch normalizeClass(target.Class) {
	case ClassPrepare:
		kind, issues = checkPrepareAlias(target.Path, workspaceRoot)
	case ClassRun:
		kind, issues = checkRunAlias(target.Path, workspaceRoot)
	default:
		return result, fmt.Errorf("alias class is required for validation")
	}

	result.Kind = kind
	result.Valid = len(issues) == 0
	result.Issues = issues
	if len(issues) > 0 {
		result.Error = issues[0].Message
	}
	return result, nil
}

type prepareDefinition struct {
	Kind  string   `yaml:"kind"`
	Image string   `yaml:"image"`
	Args  []string `yaml:"args"`
}

type runDefinition struct {
	Kind  string   `yaml:"kind"`
	Image string   `yaml:"image"`
	Args  []string `yaml:"args"`
}

const pgbenchStdinMarker = "/dev/stdin"

func checkPrepareAlias(path string, workspaceRoot string) (string, []Issue) {
	def, err := loadPrepareAlias(path)
	if err != nil {
		return "", []Issue{{Code: "invalid_prepare_alias", Message: err.Error()}}
	}
	return def.Kind, append([]Issue{}, validatePrepareAliasPaths(def.Kind, def.Args, path, workspaceRoot)...)
}

func checkRunAlias(path string, workspaceRoot string) (string, []Issue) {
	def, err := loadRunAlias(path)
	if err != nil {
		return "", []Issue{{Code: "invalid_run_alias", Message: err.Error()}}
	}
	return def.Kind, append([]Issue{}, validateRunAliasPaths(def.Kind, def.Args, path, workspaceRoot)...)
}

func loadPrepareAlias(path string) (prepareDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return prepareDefinition{}, err
	}
	var def prepareDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return prepareDefinition{}, fmt.Errorf("read prepare alias: %w", err)
	}
	def.Kind = strings.ToLower(strings.TrimSpace(def.Kind))
	switch def.Kind {
	case "":
		return prepareDefinition{}, fmt.Errorf("prepare alias kind is required")
	case "psql", "lb":
	default:
		return prepareDefinition{}, fmt.Errorf("unknown prepare alias kind: %s", def.Kind)
	}
	if len(def.Args) == 0 {
		return prepareDefinition{}, fmt.Errorf("prepare alias args are required")
	}
	return def, nil
}

func loadRunAlias(path string) (runDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runDefinition{}, err
	}
	var def runDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return runDefinition{}, fmt.Errorf("read run alias: %w", err)
	}
	def.Kind = strings.ToLower(strings.TrimSpace(def.Kind))
	switch def.Kind {
	case "":
		return runDefinition{}, fmt.Errorf("run alias kind is required")
	default:
		if !runkind.IsKnown(def.Kind) {
			return runDefinition{}, fmt.Errorf("unknown run alias kind: %s", def.Kind)
		}
	}
	if strings.TrimSpace(def.Image) != "" {
		return runDefinition{}, fmt.Errorf("run alias does not support image")
	}
	if len(def.Args) == 0 {
		return runDefinition{}, fmt.Errorf("run alias args are required")
	}
	return def, nil
}

func validatePrepareAliasPaths(kind string, args []string, aliasPath string, workspaceRoot string) []Issue {
	switch kind {
	case "psql":
		return mapInputsetIssues(inputpsql.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
	case "lb":
		return mapInputsetIssues(inputliquibase.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
	default:
		return nil
	}
}

func validateRunAliasPaths(kind string, args []string, aliasPath string, workspaceRoot string) []Issue {
	switch kind {
	case runkind.KindPsql:
		return mapInputsetIssues(inputpsql.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
	case runkind.KindPgbench:
		return mapInputsetIssues(inputpgbench.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
	default:
		return nil
	}
}

func newAliasResolver(workspaceRoot string, aliasPath string) inputset.Resolver {
	return inputset.NewAliasResolver(workspaceRoot, aliasPath)
}

func mapInputsetIssues(issues []inputset.UserError) []Issue {
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, Issue{
			Code:    issue.Code,
			Message: issue.Message,
		})
	}
	return out
}

// validatePgbenchFileArgs mirrors the pgbench runtime path semantics used by
// materializePgbenchRunArgs so alias check stays aligned with actual execution.
func validatePgbenchFileArgs(args []string, aliasPath string, workspaceRoot string) []Issue {
	return mapInputsetIssues(inputpgbench.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
}

func validateScriptFileArgs(args []string, aliasPath string, workspaceRoot string) []Issue {
	return mapInputsetIssues(inputpsql.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
}

func validateLiquibasePathArgs(args []string, aliasPath string, workspaceRoot string) []Issue {
	return mapInputsetIssues(inputliquibase.ValidateArgs(args, newAliasResolver(workspaceRoot, aliasPath), inputset.OSFileSystem{}))
}

func validateSearchPath(value string, aliasPath string, workspaceRoot string) []Issue {
	if strings.TrimSpace(value) == "" {
		return []Issue{{Code: "empty_search_path", Message: "searchPath is empty"}}
	}
	parts := strings.Split(value, ",")
	issues := make([]Issue, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			issues = append(issues, Issue{Code: "empty_search_path_item", Message: "searchPath is empty"})
			continue
		}
		if issue, ok := validateLocalLiquibaseArg(item, aliasPath, workspaceRoot, false); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func validateLocalLiquibaseArg(value string, aliasPath string, workspaceRoot string, requireFile bool) (Issue, bool) {
	if looksLikeLiquibaseRemoteRef(value) {
		return Issue{}, false
	}
	return validateLocalFileArg(value, aliasPath, workspaceRoot, requireFile)
}

func validatePgbenchFileArg(value string, aliasPath string, workspaceRoot string) (Issue, bool) {
	path, _ := splitPgbenchFileArgValue(value)
	if path == pgbenchStdinMarker {
		return Issue{}, false
	}
	return validateLocalFileArg(path, aliasPath, workspaceRoot, true)
}

func validateLocalFileArg(value string, aliasPath string, workspaceRoot string, requireFile bool) (Issue, bool) {
	cleaned := strings.TrimSpace(value)
	switch cleaned {
	case "":
		return Issue{Code: "empty_path", Message: "file path is empty"}, true
	case "-":
		return Issue{}, false
	}

	resolved, err := newAliasResolver(workspaceRoot, aliasPath).ResolvePath(cleaned)
	if err != nil {
		if issue, ok := err.(*inputset.UserError); ok {
			return Issue{
				Code:    issue.Code,
				Message: issue.Message,
				Path:    cleaned,
			}, true
		}
		return Issue{
			Code:    "invalid_path",
			Message: err.Error(),
			Path:    cleaned,
		}, true
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Issue{
			Code:    "missing_path",
			Message: fmt.Sprintf("referenced path not found: %s", cleaned),
			Path:    cleaned,
		}, true
	}
	if requireFile && info.IsDir() {
		return Issue{
			Code:    "expected_file",
			Message: fmt.Sprintf("referenced path must be a file: %s", cleaned),
			Path:    cleaned,
		}, true
	}
	return Issue{}, false
}

func looksLikeLiquibaseRemoteRef(value string) bool {
	return inputset.LooksLikeLiquibaseRemoteRef(value)
}

func splitPgbenchFileArgValue(value string) (string, string) {
	return inputset.SplitPgbenchFileArgValue(value)
}
