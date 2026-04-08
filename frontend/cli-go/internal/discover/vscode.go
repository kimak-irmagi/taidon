package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AnalyzeVSCode reports missing or incomplete VS Code workspace guidance for
// sqlrs-related YAML schema settings.
func AnalyzeVSCode(opts Options) (Report, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Report{}, fmt.Errorf("workspace root is required for discover")
	}
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return Report{}, err
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	targetAbs := filepath.Join(workspaceRoot, ".vscode", "settings.json")
	targetRel := filepath.ToSlash(filepath.Join(".vscode", "settings.json"))
	merged, changed, err := mergeVSCodeSettingsFile(targetAbs)
	if err != nil {
		return Report{
			SelectedAnalyzers: []string{AnalyzerVSCode},
			Findings: []Finding{{
				Analyzer: AnalyzerVSCode,
				Target:   targetRel,
				Action:   "inspect VS Code settings manually",
				Error:    err.Error(),
				Valid:    false,
			}},
		}, nil
	}
	if !changed {
		return Report{SelectedAnalyzers: []string{AnalyzerVSCode}}, nil
	}
	command := renderJSONWriteCommand(normalizedShellFamily(opts.ShellFamily), targetRel, merged)
	return Report{
		SelectedAnalyzers: []string{AnalyzerVSCode},
		Findings: []Finding{{
			Analyzer:        AnalyzerVSCode,
			Target:          targetRel,
			Action:          "add missing VS Code yaml schema guidance",
			Reason:          "workspace settings should associate .sqlrs/config.yaml with the sqlrs schema",
			JSONPayload:     string(merged),
			FollowUpCommand: &FollowUpCommand{ShellFamily: normalizedShellFamily(opts.ShellFamily), Command: command},
			CreateCommand:   command,
			Valid:           true,
		}},
	}, nil
}

func mergeVSCodeSettingsFile(path string) ([]byte, bool, error) {
	var current map[string]any
	data, err := osReadFile(path)
	if err != nil {
		return nil, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		current = map[string]any{}
	} else if err := json.Unmarshal(data, &current); err != nil {
		return nil, false, err
	}
	changed := ensureVSCodeSchemaMapping(current)
	if !changed {
		return nil, false, nil
	}
	merged, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(merged, '\n'), true, nil
}

var osReadFile = func(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func ensureVSCodeSchemaMapping(root map[string]any) bool {
	if root == nil {
		return false
	}
	changed := false
	yamlSchemas, ok := root["yaml.schemas"].(map[string]any)
	if !ok {
		yamlSchemas = map[string]any{}
		root["yaml.schemas"] = yamlSchemas
		changed = true
	}
	const schemaPath = "./.vscode/sqlrs-workspace-config.schema.json"
	const configGlob = "**/.sqlrs/config.yaml"
	raw, ok := yamlSchemas[schemaPath]
	if !ok {
		yamlSchemas[schemaPath] = []any{configGlob}
		return true
	}
	switch values := raw.(type) {
	case []any:
		for _, value := range values {
			if text, ok := value.(string); ok && text == configGlob {
				return changed
			}
		}
		yamlSchemas[schemaPath] = append(values, configGlob)
		return true
	case []string:
		for _, value := range values {
			if value == configGlob {
				return changed
			}
		}
		yamlSchemas[schemaPath] = append(values, configGlob)
		return true
	default:
		yamlSchemas[schemaPath] = []any{configGlob}
		return true
	}
}

func renderJSONWriteCommand(shellFamily string, target string, content []byte) string {
	payload := string(content)
	switch normalizedShellFamily(shellFamily) {
	case ShellFamilyPowerShell:
		return fmt.Sprintf("$content = @'\n%s'@; New-Item -ItemType Directory -Force -Path %s | Out-Null; Set-Content -Path %s -Value $content", payload, shellQuoteForGoOS("windows", filepath.ToSlash(filepath.Dir(target))), shellQuoteForGoOS("windows", target))
	default:
		return fmt.Sprintf("mkdir -p %s && cat <<'EOF' > %s\n%sEOF", shellQuoteForGoOS("linux", filepath.ToSlash(filepath.Dir(target))), shellQuoteForGoOS("linux", target), payload)
	}
}
