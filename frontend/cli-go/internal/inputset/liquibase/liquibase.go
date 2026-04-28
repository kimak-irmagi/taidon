// Package liquibase owns the shared Liquibase file-bearing semantics described in
// docs/architecture/inputset-component-structure.md.
package liquibase

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sqlrs/cli/internal/inputset"
	"gopkg.in/yaml.v3"
)

type boolish string

func (b *boolish) UnmarshalYAML(node *yaml.Node) error {
	*b = boolish(node.Value)
	return nil
}

func (b *boolish) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("true")) || bytes.Equal(data, []byte("false")) {
		*b = boolish(string(data))
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*b = boolish(s)
	return nil
}

func (b boolish) useChangelogDir() bool {
	return !strings.EqualFold(strings.TrimSpace(string(b)), "false")
}

type xmlChangeLog struct {
	XMLName    xml.Name      `xml:"databaseChangeLog"`
	Includes   []includeSpec `xml:"include"`
	IncludeAll []includeAll  `xml:"includeAll"`
}

type structuredChangeLog struct {
	DatabaseChangeLog []changeItem `yaml:"databaseChangeLog" json:"databaseChangeLog"`
}

type changeItem struct {
	Include    *includeSpec `yaml:"include" json:"include"`
	IncludeAll *includeAll  `yaml:"includeAll" json:"includeAll"`
}

type includeSpec struct {
	File                    string  `xml:"file,attr" yaml:"file" json:"file"`
	RelativeToChangelogFile boolish `xml:"relativeToChangelogFile,attr" yaml:"relativeToChangelogFile" json:"relativeToChangelogFile"`
}

type includeAll struct {
	Path                    string  `xml:"path,attr" yaml:"path" json:"path"`
	RelativeToChangelogFile boolish `xml:"relativeToChangelogFile,attr" yaml:"relativeToChangelogFile" json:"relativeToChangelogFile"`
}

type declaredPath struct {
	flag        string
	path        string
	requireFile bool
}

// NormalizeArgs applies shared Liquibase path-bearing normalization.
func NormalizeArgs(args []string, resolver inputset.Resolver, canonicalSearchPath bool) ([]string, error) {
	normalized := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file" || arg == "--searchPath" || arg == "--search-path":
			if i+1 >= len(args) {
				return nil, inputset.Errorf("missing_path_arg", "Missing value for %s", arg)
			}
			flag := normalizeFlag(arg, canonicalSearchPath)
			value, err := rewriteValue(flag, args[i+1], resolver)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, flag, value)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value, err := rewriteValue("--changelog-file", strings.TrimPrefix(arg, "--changelog-file="), resolver)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--changelog-file="+value)
		case strings.HasPrefix(arg, "--defaults-file="):
			value, err := rewriteValue("--defaults-file", strings.TrimPrefix(arg, "--defaults-file="), resolver)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--defaults-file="+value)
		case strings.HasPrefix(arg, "--searchPath="):
			value, err := rewriteValue("--searchPath", strings.TrimPrefix(arg, "--searchPath="), resolver)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, "--searchPath="+value)
		case strings.HasPrefix(arg, "--search-path="):
			value, err := rewriteValue("--searchPath", strings.TrimPrefix(arg, "--search-path="), resolver)
			if err != nil {
				return nil, err
			}
			if canonicalSearchPath {
				normalized = append(normalized, "--searchPath="+value)
			} else {
				normalized = append(normalized, "--search-path="+value)
			}
		default:
			normalized = append(normalized, arg)
		}
	}

	return normalized, nil
}

// Collect builds the deterministic Liquibase direct-input and changelog-graph set.
func Collect(args []string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.InputSet, error) {
	declared, searchPaths, changelogPath, err := collectDeclared(args, resolver)
	if err != nil {
		return inputset.InputSet{}, err
	}
	if changelogPath == "" {
		return inputset.InputSet{}, fmt.Errorf("liquibase command has no --changelog-file (required for diff)")
	}
	return collectInputSet(declared, searchPaths, resolver, fs)
}

// CollectInvocationInputs builds the deterministic Liquibase input set for a
// concrete invocation and includes the changelog graph when a changelog file is
// declared.
func CollectInvocationInputs(args []string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.InputSet, error) {
	declared, searchPaths, _, err := collectDeclared(args, resolver)
	if err != nil {
		return inputset.InputSet{}, err
	}
	return collectInputSet(declared, searchPaths, resolver, fs)
}

func collectInputSet(declared []declaredPath, searchPaths []string, resolver inputset.Resolver, fs inputset.FileSystem) (inputset.InputSet, error) {
	tracker := &tracker{
		fs:          fs,
		root:        resolver.Root,
		searchPaths: searchPaths,
		seen:        make(map[string]struct{}),
	}
	var order []string
	for _, ref := range declared {
		if ref.flag == "--changelog-file" {
			if err := tracker.collect(ref.path, &order); err != nil {
				return inputset.InputSet{}, err
			}
			continue
		}
		if err := tracker.addLeaf(ref.path, &order); err != nil {
			return inputset.InputSet{}, err
		}
	}

	return buildInputSet(order, resolver.Root, fs)
}

func buildInputSet(order []string, root string, fs inputset.FileSystem) (inputset.InputSet, error) {
	entries := make([]inputset.InputEntry, 0, len(order))
	for _, path := range order {
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		var hash string
		if oid, ok := fs.(inputset.BlobOIDer); ok {
			h, err := oid.BlobOID(path)
			if err != nil {
				return inputset.InputSet{}, fmt.Errorf("hash %s: %w", path, err)
			}
			hash = h
		} else {
			content, err := fs.ReadFile(path)
			if err != nil {
				return inputset.InputSet{}, fmt.Errorf("read %s: %w", path, err)
			}
			hash = inputset.HashContent(content)
		}
		entries = append(entries, inputset.InputEntry{
			Path:    rel,
			AbsPath: path,
			Hash:    hash,
		})
	}
	return inputset.InputSet{Entries: entries}, nil
}

// ValidateArgs accumulates alias-check issues for the shared Liquibase path syntax.
func ValidateArgs(args []string, resolver inputset.Resolver, fs inputset.FileSystem) []inputset.UserError {
	issues := make([]inputset.UserError, 0, 2)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file":
			if i+1 >= len(args) {
				issues = append(issues, *inputset.Errorf("missing_path_arg", "missing value for %s", arg))
				continue
			}
			if issue, ok := validateLocalArg(args[i+1], resolver, fs, true); ok {
				issues = append(issues, issue)
			}
			i++
		case arg == "--searchPath" || arg == "--search-path":
			if i+1 >= len(args) {
				issues = append(issues, *inputset.Errorf("missing_path_arg", "missing value for %s", arg))
				continue
			}
			issues = append(issues, validateSearchPath(args[i+1], resolver, fs)...)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			if issue, ok := validateLocalArg(strings.TrimPrefix(arg, "--changelog-file="), resolver, fs, true); ok {
				issues = append(issues, issue)
			}
		case strings.HasPrefix(arg, "--defaults-file="):
			if issue, ok := validateLocalArg(strings.TrimPrefix(arg, "--defaults-file="), resolver, fs, true); ok {
				issues = append(issues, issue)
			}
		case strings.HasPrefix(arg, "--searchPath="):
			issues = append(issues, validateSearchPath(strings.TrimPrefix(arg, "--searchPath="), resolver, fs)...)
		case strings.HasPrefix(arg, "--search-path="):
			issues = append(issues, validateSearchPath(strings.TrimPrefix(arg, "--search-path="), resolver, fs)...)
		}
	}
	return issues
}

func normalizeFlag(flag string, canonicalSearchPath bool) string {
	if canonicalSearchPath && flag == "--search-path" {
		return "--searchPath"
	}
	return flag
}

func rewriteValue(flag string, value string, resolver inputset.Resolver) (string, error) {
	if flag == "--searchPath" || flag == "--search-path" {
		return rewriteSearchPath(value, resolver)
	}
	if strings.TrimSpace(value) == "" {
		return "", inputset.Errorf("empty_path", "Path is empty")
	}
	if inputset.LooksLikeLiquibaseRemoteRef(value) {
		return value, nil
	}
	return resolver.ResolvePath(value)
}

func rewriteSearchPath(value string, resolver inputset.Resolver) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", inputset.Errorf("empty_search_path", "searchPath is empty")
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			return "", inputset.Errorf("empty_search_path", "searchPath is empty")
		}
		if inputset.LooksLikeLiquibaseRemoteRef(item) {
			out = append(out, item)
			continue
		}
		path, err := resolver.ResolvePath(item)
		if err != nil {
			return "", err
		}
		out = append(out, path)
	}
	return strings.Join(out, ","), nil
}

func collectDeclared(args []string, resolver inputset.Resolver) ([]declaredPath, []string, string, error) {
	declared := make([]declaredPath, 0, 2)
	searchPaths := make([]string, 0, 2)
	changelogPath := ""

	appendSearchPaths := func(value string) error {
		parts, err := resolveSearchPathParts(value, resolver)
		if err != nil {
			return err
		}
		searchPaths = append(searchPaths, parts...)
		return nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file":
			if i+1 >= len(args) {
				return nil, nil, "", inputset.Errorf("missing_path_arg", "Missing value for %s", arg)
			}
			value, err := rewriteValue(arg, args[i+1], resolver)
			if err != nil {
				return nil, nil, "", err
			}
			if !inputset.LooksLikeLiquibaseRemoteRef(value) {
				declared = append(declared, declaredPath{flag: arg, path: value, requireFile: true})
				if arg == "--changelog-file" {
					changelogPath = value
				}
			}
			i++
		case arg == "--searchPath" || arg == "--search-path":
			if i+1 >= len(args) {
				return nil, nil, "", inputset.Errorf("missing_path_arg", "Missing value for %s", arg)
			}
			if err := appendSearchPaths(args[i+1]); err != nil {
				return nil, nil, "", err
			}
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value, err := rewriteValue("--changelog-file", strings.TrimPrefix(arg, "--changelog-file="), resolver)
			if err != nil {
				return nil, nil, "", err
			}
			if !inputset.LooksLikeLiquibaseRemoteRef(value) {
				declared = append(declared, declaredPath{flag: "--changelog-file", path: value, requireFile: true})
				changelogPath = value
			}
		case strings.HasPrefix(arg, "--defaults-file="):
			value, err := rewriteValue("--defaults-file", strings.TrimPrefix(arg, "--defaults-file="), resolver)
			if err != nil {
				return nil, nil, "", err
			}
			if !inputset.LooksLikeLiquibaseRemoteRef(value) {
				declared = append(declared, declaredPath{flag: "--defaults-file", path: value, requireFile: true})
			}
		case strings.HasPrefix(arg, "--searchPath="):
			if err := appendSearchPaths(strings.TrimPrefix(arg, "--searchPath=")); err != nil {
				return nil, nil, "", err
			}
		case strings.HasPrefix(arg, "--search-path="):
			if err := appendSearchPaths(strings.TrimPrefix(arg, "--search-path=")); err != nil {
				return nil, nil, "", err
			}
		}
	}
	return declared, searchPaths, changelogPath, nil
}

func resolveSearchPathParts(value string, resolver inputset.Resolver) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, inputset.Errorf("empty_search_path", "searchPath is empty")
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			return nil, inputset.Errorf("empty_search_path", "searchPath is empty")
		}
		if inputset.LooksLikeLiquibaseRemoteRef(item) {
			continue
		}
		path, err := resolver.ResolvePath(item)
		if err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}

type tracker struct {
	fs          inputset.FileSystem
	root        string
	searchPaths []string
	seen        map[string]struct{}
}

func (t *tracker) addLeaf(path string, order *[]string) error {
	path = filepath.Clean(path)
	if _, ok := t.seen[path]; ok {
		return nil
	}
	if _, err := t.fs.Stat(path); err != nil {
		return err
	}
	t.seen[path] = struct{}{}
	*order = append(*order, path)
	return nil
}

func (t *tracker) collect(path string, order *[]string) error {
	path = filepath.Clean(path)
	if _, ok := t.seen[path]; ok {
		return nil
	}
	if _, err := t.fs.Stat(path); err != nil {
		return err
	}
	t.seen[path] = struct{}{}
	*order = append(*order, path)

	content, err := t.fs.ReadFile(path)
	if err != nil {
		return err
	}
	includes, includeAlls, err := parseIncludes(path, content)
	if err != nil {
		return err
	}

	changelogDir := filepath.Dir(path)
	for _, inc := range includes {
		next := t.resolveIncludePath(changelogDir, inc.File, inc.RelativeToChangelogFile.useChangelogDir())
		if next == "" {
			continue
		}
		if err := t.collect(next, order); err != nil {
			return err
		}
	}
	for _, inc := range includeAlls {
		dir := t.resolveIncludePath(changelogDir, inc.Path, inc.RelativeToChangelogFile.useChangelogDir())
		if dir == "" {
			continue
		}
		if err := t.collectIncludeAll(dir, order); err != nil {
			return err
		}
	}

	return nil
}

func (t *tracker) collectIncludeAll(dir string, order *[]string) error {
	entries, err := t.fs.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("includeAll %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".sql") ||
			strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
			strings.HasSuffix(lower, ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := t.collect(filepath.Join(dir, name), order); err != nil {
			return err
		}
	}
	return nil
}

func (t *tracker) resolveIncludePath(changelogDir string, raw string, relativeToChangelog bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if inputset.LooksLikeLiquibaseRemoteRef(raw) {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if relativeToChangelog {
		return filepath.Clean(filepath.Join(changelogDir, filepath.FromSlash(raw)))
	}
	for _, base := range t.searchPaths {
		candidate := filepath.Clean(filepath.Join(base, filepath.FromSlash(raw)))
		if _, err := t.fs.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Clean(filepath.Join(t.root, filepath.FromSlash(raw)))
}

func parseIncludes(path string, content []byte) ([]includeSpec, []includeAll, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xml":
		var log xmlChangeLog
		if err := xml.Unmarshal(content, &log); err != nil {
			return nil, nil, fmt.Errorf("parse liquibase changelog %s: %w", path, err)
		}
		return log.Includes, log.IncludeAll, nil
	case ".yaml", ".yml":
		var log structuredChangeLog
		if err := yaml.Unmarshal(content, &log); err != nil {
			return nil, nil, fmt.Errorf("parse liquibase changelog %s: %w", path, err)
		}
		return flattenIncludes(log.DatabaseChangeLog), flattenIncludeAll(log.DatabaseChangeLog), nil
	case ".json":
		var log structuredChangeLog
		if err := json.Unmarshal(content, &log); err != nil {
			return nil, nil, fmt.Errorf("parse liquibase changelog %s: %w", path, err)
		}
		return flattenIncludes(log.DatabaseChangeLog), flattenIncludeAll(log.DatabaseChangeLog), nil
	default:
		return nil, nil, nil
	}
}

func flattenIncludes(items []changeItem) []includeSpec {
	out := make([]includeSpec, 0, len(items))
	for _, item := range items {
		if item.Include != nil {
			out = append(out, *item.Include)
		}
	}
	return out
}

func flattenIncludeAll(items []changeItem) []includeAll {
	out := make([]includeAll, 0, len(items))
	for _, item := range items {
		if item.IncludeAll != nil {
			out = append(out, *item.IncludeAll)
		}
	}
	return out
}

func validateSearchPath(value string, resolver inputset.Resolver, fs inputset.FileSystem) []inputset.UserError {
	if strings.TrimSpace(value) == "" {
		return []inputset.UserError{*inputset.Errorf("empty_search_path", "searchPath is empty")}
	}
	parts := strings.Split(value, ",")
	issues := make([]inputset.UserError, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			issues = append(issues, *inputset.Errorf("empty_search_path_item", "searchPath is empty"))
			continue
		}
		if issue, ok := validateLocalArg(item, resolver, fs, false); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func validateLocalArg(value string, resolver inputset.Resolver, fs inputset.FileSystem, requireFile bool) (inputset.UserError, bool) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return *inputset.Errorf("empty_path", "file path is empty"), true
	}
	if inputset.LooksLikeLiquibaseRemoteRef(cleaned) {
		return inputset.UserError{}, false
	}
	resolved, err := resolver.ResolvePath(cleaned)
	if err != nil {
		if issue, ok := err.(*inputset.UserError); ok {
			return *issue, true
		}
		return inputset.UserError{Code: "invalid_path", Message: err.Error()}, true
	}
	info, err := fs.Stat(resolved)
	if err != nil {
		return *inputset.Errorf("missing_path", "referenced path not found: %s", cleaned), true
	}
	if requireFile && info.IsDir() {
		return *inputset.Errorf("expected_file", "referenced path must be a file: %s", cleaned), true
	}
	return inputset.UserError{}, false
}
