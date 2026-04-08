package discover

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// AnalyzePrepareShaping reports simple local workflow split opportunities that
// could improve prepare reuse.
func AnalyzePrepareShaping(opts Options) (Report, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Report{}, fmt.Errorf("workspace root is required for discover")
	}
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return Report{}, err
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	type bucket struct {
		stable   []string
		volatile []string
	}
	byDir := make(map[string]bucket)
	err = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".sqlrs", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".sql", ".xml", ".yaml", ".yml", ".json":
		default:
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		dir := filepath.Dir(path)
		group := byDir[dir]
		switch {
		case strings.Contains(name, "schema"), strings.Contains(name, "init"), strings.Contains(name, "ddl"), strings.Contains(name, "base"):
			group.stable = append(group.stable, path)
		case strings.Contains(name, "seed"), strings.Contains(name, "demo"), strings.Contains(name, "sample"), strings.Contains(name, "data"):
			group.volatile = append(group.volatile, path)
		}
		byDir[dir] = group
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	report := Report{
		SelectedAnalyzers: []string{AnalyzerPrepareShaping},
		Findings:          make([]Finding, 0),
	}
	for dir, group := range byDir {
		if len(group.stable) == 0 || len(group.volatile) == 0 {
			continue
		}
		targetRel, ok := stableDiscoverRelativePath(workspaceRoot, dir, true)
		if !ok {
			targetRel = filepath.ToSlash(dir)
		}
		report.Findings = append(report.Findings, Finding{
			Analyzer: AnalyzerPrepareShaping,
			Target:   targetRel,
			Action:   "split stable base preparation from volatile seed/demo inputs",
			Reason:   "the same directory mixes stable schema/bootstrap inputs with volatile seed/demo inputs",
			Valid:    true,
		})
	}
	return report, nil
}
