package discover

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
)

const (
	// AnalyzerAliases keeps the existing alias discovery analyzer in the stable
	// discover registry defined by docs/architecture/discover-component-structure.md.
	AnalyzerAliases        = "aliases"
	AnalyzerGitignore      = "gitignore"
	AnalyzerVSCode         = "vscode"
	AnalyzerPrepareShaping = "prepare-shaping"

	ShellFamilyPOSIX      = "posix"
	ShellFamilyPowerShell = "powershell"
)

var stableAnalyzers = []string{
	AnalyzerAliases,
	AnalyzerGitignore,
	AnalyzerVSCode,
	AnalyzerPrepareShaping,
}

type analyzerRunner func(Options) (Report, error)

var analyzerRegistry = map[string]analyzerRunner{
	AnalyzerAliases:        AnalyzeAliases,
	AnalyzerGitignore:      AnalyzeGitignore,
	AnalyzerVSCode:         AnalyzeVSCode,
	AnalyzerPrepareShaping: AnalyzePrepareShaping,
}

// NormalizeSelectedAnalyzers applies the discover analyzer selection rules:
// deduplicate, keep only known analyzers, and preserve canonical order.
func NormalizeSelectedAnalyzers(selected []string) ([]string, error) {
	if len(selected) == 0 {
		return slices.Clone(stableAnalyzers), nil
	}

	seen := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		name := strings.ToLower(strings.TrimSpace(value))
		if name == "" {
			continue
		}
		if _, ok := analyzerRegistry[name]; !ok {
			return nil, fmt.Errorf("unknown discover option: --%s", name)
		}
		seen[name] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for _, name := range stableAnalyzers {
		if _, ok := seen[name]; ok {
			result = append(result, name)
		}
	}
	return result, nil
}

func defaultShellFamily() string {
	if runtime.GOOS == "windows" {
		return ShellFamilyPowerShell
	}
	return ShellFamilyPOSIX
}

func normalizedShellFamily(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ShellFamilyPowerShell:
		return ShellFamilyPowerShell
	case ShellFamilyPOSIX:
		return ShellFamilyPOSIX
	default:
		return defaultShellFamily()
	}
}
