package alias

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scan walks the bounded workspace slice defined by the approved alias
// inspection flow in docs/architecture/alias-inspection-flow.md.
func Scan(opts ScanOptions) ([]Entry, error) {
	workspaceRoot, cwd, scanRoot, depth, classSet, err := normalizeScanRequest(opts)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, 16)
	if err := walkDirectory(scanRoot, 0, depth, func(path string) error {
		class := classifyPath(path)
		if class == "" {
			return nil
		}
		if _, ok := classSet[class]; !ok {
			return nil
		}
		entry := Entry{
			Class: class,
			Ref:   invocationRef(path, cwd, class),
			File:  workspaceRelativePath(path, workspaceRoot),
			Path:  path,
		}
		kind, err := readInventoryKind(path, class)
		if err != nil {
			entry.Status = "invalid"
			entry.Error = err.Error()
		} else {
			entry.Status = "ok"
			entry.Kind = kind
		}
		entries = append(entries, entry)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].File < entries[j].File
	})
	return entries, nil
}

func normalizeScanRequest(opts ScanOptions) (string, string, string, Depth, map[Class]struct{}, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return "", "", "", "", nil, fmt.Errorf("workspace root is required for alias inspection")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd = workspaceRoot
	}
	cwd = filepath.Clean(cwd)

	scanRoot := workspaceRoot
	switch from := strings.TrimSpace(opts.From); strings.ToLower(from) {
	case "", "cwd":
		if !isWithin(canonicalizeBoundaryPath(workspaceRoot), canonicalizeBoundaryPath(cwd)) {
			return "", "", "", "", nil, fmt.Errorf("current working directory must stay within workspace root")
		}
		scanRoot = cwd
	case "workspace":
		scanRoot = workspaceRoot
	default:
		if filepath.IsAbs(from) {
			scanRoot = filepath.Clean(from)
		} else {
			if !isWithin(canonicalizeBoundaryPath(workspaceRoot), canonicalizeBoundaryPath(cwd)) {
				return "", "", "", "", nil, fmt.Errorf("current working directory must stay within workspace root")
			}
			scanRoot = filepath.Clean(filepath.Join(cwd, filepath.FromSlash(from)))
		}
	}

	if !isWithin(canonicalizeBoundaryPath(workspaceRoot), canonicalizeBoundaryPath(scanRoot)) {
		return "", "", "", "", nil, fmt.Errorf("scan root must stay within workspace root: %s", scanRoot)
	}

	depth := normalizeDepth(opts.Depth)
	if depth == "" {
		return "", "", "", "", nil, fmt.Errorf("invalid scan depth: %s", strings.TrimSpace(opts.Depth))
	}

	classSet := map[Class]struct{}{}
	for _, class := range normalizeClasses(opts.Classes) {
		classSet[class] = struct{}{}
	}
	return workspaceRoot, cwd, scanRoot, depth, classSet, nil
}

func walkDirectory(dir string, level int, depth Depth, visit func(string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			if name == ".sqlrs" {
				continue
			}
			switch depth {
			case DepthSelf:
				continue
			case DepthChildren:
				if level >= 1 {
					continue
				}
			}
			if err := walkDirectory(path, level+1, depth, visit); err != nil {
				return err
			}
			continue
		}
		if err := visit(path); err != nil {
			return err
		}
	}
	return nil
}

func readInventoryKind(path string, class Class) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var header struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return "", inventoryReadError(class, err)
	}
	return strings.ToLower(strings.TrimSpace(header.Kind)), nil
}

func inventoryReadError(class Class, err error) error {
	switch class {
	case ClassPrepare:
		return fmt.Errorf("read prepare alias: %w", err)
	case ClassRun:
		return fmt.Errorf("read run alias: %w", err)
	default:
		return err
	}
}

func workspaceRelativePath(path string, workspaceRoot string) string {
	rel, err := portableRelativePath(workspaceRoot, path)
	if err != nil {
		return filepath.ToSlash(portablePathBase(path))
	}
	return filepath.ToSlash(rel)
}

func invocationRef(path string, cwd string, class Class) string {
	rel, err := portableRelativePath(cwd, path)
	if err != nil {
		rel = portablePathBase(path)
	}
	switch class {
	case ClassPrepare:
		rel = strings.TrimSuffix(rel, prepareSuffix)
	case ClassRun:
		rel = strings.TrimSuffix(rel, runSuffix)
	}
	return filepath.ToSlash(rel)
}

func classifyPath(path string) Class {
	switch {
	case strings.HasSuffix(strings.ToLower(path), prepareSuffix):
		return ClassPrepare
	case strings.HasSuffix(strings.ToLower(path), runSuffix):
		return ClassRun
	default:
		return ""
	}
}

func portableRelativePath(base string, target string) (string, error) {
	// Prefer the canonical form first so symlinked paths collapse to a stable relative path.
	canonicalBase := canonicalizeBoundaryPath(base)
	canonicalTarget := canonicalizeBoundaryPath(target)
	if canonicalBase != "" && canonicalTarget != "" {
		if canonicalRel, canonicalErr := filepath.Rel(canonicalBase, canonicalTarget); canonicalErr == nil {
			return canonicalRel, nil
		}
	}

	baseVolume := windowsVolumeName(base)
	targetVolume := windowsVolumeName(target)
	if baseVolume != "" || targetVolume != "" {
		if baseVolume == "" || targetVolume == "" || !strings.EqualFold(baseVolume, targetVolume) {
			return "", fmt.Errorf("paths have different roots")
		}
		return slashRelativePath(normalizeWindowsLikePath(base[len(baseVolume):]), normalizeWindowsLikePath(target[len(targetVolume):])), nil
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func windowsVolumeName(value string) string {
	if len(value) < 2 || value[1] != ':' {
		return ""
	}
	letter := value[0]
	if (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') {
		return value[:2]
	}
	return ""
}

func normalizeWindowsLikePath(value string) string {
	normalized := strings.ReplaceAll(value, `\`, "/")
	if normalized == "" {
		return "/"
	}
	cleaned := path.Clean(normalized)
	if strings.HasPrefix(cleaned, "/") {
		return cleaned
	}
	return "/" + strings.TrimPrefix(cleaned, "/")
}

func slashRelativePath(base string, target string) string {
	baseParts := splitSlashPath(base)
	targetParts := splitSlashPath(target)

	shared := 0
	for shared < len(baseParts) && shared < len(targetParts) {
		if !strings.EqualFold(baseParts[shared], targetParts[shared]) {
			break
		}
		shared++
	}

	parts := make([]string, 0, len(baseParts)-shared+len(targetParts)-shared)
	for i := shared; i < len(baseParts); i++ {
		parts = append(parts, "..")
	}
	parts = append(parts, targetParts[shared:]...)
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

func splitSlashPath(value string) []string {
	trimmed := strings.TrimPrefix(value, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func portablePathBase(value string) string {
	trimmed := strings.TrimRight(value, `/\`)
	if trimmed == "" {
		return ""
	}
	if idx := strings.LastIndexAny(trimmed, `/\`); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}
