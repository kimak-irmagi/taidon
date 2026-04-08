package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type gitignoreArtifact struct {
	AbsPath string
	Target  string
	Entry   string
	Reason  string
}

// AnalyzeGitignore reports missing ignore coverage for local-only workspace
// artifacts such as .sqlrs/ and local coverage snapshots.
func AnalyzeGitignore(opts Options) (Report, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Report{}, fmt.Errorf("workspace root is required for discover")
	}
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return Report{}, err
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	artifacts, err := discoverGitignoreArtifacts(workspaceRoot)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		SelectedAnalyzers: []string{AnalyzerGitignore},
		Findings:          make([]Finding, 0, len(artifacts)),
	}
	for _, artifact := range artifacts {
		targetAbs := chooseGitignoreTarget(workspaceRoot, artifact.Target)
		targetRel, ok := stableDiscoverRelativePath(workspaceRoot, targetAbs, true)
		if !ok {
			targetRel = filepath.ToSlash(targetAbs)
		}
		missing, err := gitignoreMissingEntries(targetAbs, []string{artifact.Entry})
		if err != nil {
			report.Findings = append(report.Findings, Finding{
				Analyzer: AnalyzerGitignore,
				Target:   targetRel,
				Action:   "inspect gitignore coverage manually",
				Reason:   artifact.Reason,
				Error:    err.Error(),
				Valid:    false,
			})
			continue
		}
		if len(missing) == 0 {
			continue
		}
		report.Findings = append(report.Findings, Finding{
			Analyzer:         AnalyzerGitignore,
			Target:           targetRel,
			Action:           "add missing ignore entries",
			Reason:           artifact.Reason,
			FollowUpCommand:  &FollowUpCommand{ShellFamily: normalizedShellFamily(opts.ShellFamily), Command: renderGitignoreCommand(normalizedShellFamily(opts.ShellFamily), targetRel, missing)},
			CreateCommand:    renderGitignoreCommand(normalizedShellFamily(opts.ShellFamily), targetRel, missing),
			SuggestedEntries: slices.Clone(missing),
			Valid:            true,
		})
	}
	return report, nil
}

func discoverGitignoreArtifacts(workspaceRoot string) ([]gitignoreArtifact, error) {
	result := make([]gitignoreArtifact, 0, 4)
	sqlrsDir := filepath.Join(workspaceRoot, ".sqlrs")
	if info, err := os.Stat(sqlrsDir); err == nil && info.IsDir() {
		result = append(result, gitignoreArtifact{
			AbsPath: sqlrsDir,
			Target:  workspaceRoot,
			Entry:   ".sqlrs/",
			Reason:  "local sqlrs workspace state should stay out of version control",
		})
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
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
		if strings.EqualFold(d.Name(), "coverage-current") {
			result = append(result, gitignoreArtifact{
				AbsPath: path,
				Target:  filepath.Dir(path),
				Entry:   "coverage-current",
				Reason:  "local coverage snapshots should stay out of version control",
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func chooseGitignoreTarget(workspaceRoot string, preferredDir string) string {
	dir := filepath.Clean(preferredDir)
	root := filepath.Clean(workspaceRoot)
	if samePath(dir, root) {
		return filepath.Join(root, ".gitignore")
	}
	return filepath.Join(dir, ".gitignore")
}

func gitignoreMissingEntries(path string, entries []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	missing := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !gitignoreContainsEntry(lines, entry) {
			missing = append(missing, entry)
		}
	}
	return missing, nil
}

func gitignoreContainsEntry(lines []string, entry string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(entry) {
			return true
		}
	}
	return false
}

func renderGitignoreCommand(shellFamily string, target string, entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	switch normalizedShellFamily(shellFamily) {
	case ShellFamilyPowerShell:
		parts := make([]string, 0, len(entries))
		targetQuoted := shellQuoteForGoOS("windows", target)
		for _, entry := range entries {
			entryQuoted := shellQuoteForGoOS("windows", entry)
			parts = append(parts, fmt.Sprintf("if (-not (Select-String -Path %s -SimpleMatch %s -Quiet -ErrorAction SilentlyContinue)) { Add-Content -Path %s -Value %s }", targetQuoted, entryQuoted, targetQuoted, entryQuoted))
		}
		return strings.Join(parts, "; ")
	default:
		parts := make([]string, 0, len(entries))
		targetQuoted := shellQuoteForGoOS("linux", target)
		for _, entry := range entries {
			entryQuoted := shellQuoteForGoOS("linux", entry)
			parts = append(parts, fmt.Sprintf("grep -qxF %s %s 2>/dev/null || printf '%%s\\n' %s >> %s", entryQuoted, targetQuoted, entryQuoted, targetQuoted))
		}
		return strings.Join(parts, " && ")
	}
}

func samePath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
