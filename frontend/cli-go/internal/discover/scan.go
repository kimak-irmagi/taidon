package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/pathutil"
)

const discoverScanHeartbeat = 64

func walkDiscoverFiles(workspaceRoot string, cwd string, progress Progress) ([]fileRecord, int, error) {
	records := make([]fileRecord, 0, 32)
	scanned := 0
	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".sqlrs", ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		scanned++
		if progress != nil && scanned%discoverScanHeartbeat == 0 {
			emitProgress(progress, ProgressEvent{
				Stage:   ProgressStageScanProgress,
				Scanned: scanned,
			})
		}
		lowerName := strings.ToLower(name)
		if strings.HasSuffix(lowerName, ".prep.s9s.yaml") || strings.HasSuffix(lowerName, ".run.s9s.yaml") {
			return nil
		}
		record, ok := classifyDiscoverFile(workspaceRoot, cwd, path)
		if !ok {
			return nil
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if progress != nil && scanned > 0 {
		emitProgress(progress, ProgressEvent{
			Stage:   ProgressStageScanProgress,
			Scanned: scanned,
		})
	}
	return records, scanned, nil
}

func classifyDiscoverFile(workspaceRoot string, cwd string, path string) (fileRecord, bool) {
	relWorkspace, ok := stableDiscoverRelativePath(workspaceRoot, path, false)
	if !ok {
		return fileRecord{}, false
	}
	relCWD, ok := stableDiscoverRelativePath(cwd, path, true)
	if !ok {
		// Keep the full path when a cwd-relative path cannot be formed
		// (for example, when cwd and the workspace live on different drives).
		relCWD = filepath.ToSlash(path)
	}

	lowerPath := strings.ToLower(filepath.ToSlash(relWorkspace))
	lowerBase := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if !isSupportedDiscoverExtension(ext) {
		return fileRecord{}, false
	}
	binaryOnly := ext == ".class" || ext == ".jar"

	record := fileRecord{
		AbsPath:       path,
		WorkspaceRoot: workspaceRoot,
		WorkspaceRel:  filepath.ToSlash(relWorkspace),
		CwdRel:        filepath.ToSlash(relCWD),
		Ext:           ext,
		LowerPath:     lowerPath,
		LowerBase:     lowerBase,
		BinaryOnly:    binaryOnly,
	}

	if !binaryOnly {
		content, err := readDiscoverSnippet(path)
		if err != nil {
			return fileRecord{}, false
		}
		record.Content = strings.ToLower(content)
	}

	return record, true
}

// stableDiscoverRelativePath keeps discover output stable when the same
// workspace is reachable through different symlinked path forms.
func stableDiscoverRelativePath(base string, target string, fallbackAbsolute bool) (string, bool) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		if fallbackAbsolute {
			return filepath.ToSlash(target), true
		}
		return "", false
	}

	canonicalBase := pathutil.CanonicalizeBoundaryPath(base)
	canonicalTarget := pathutil.CanonicalizeBoundaryPath(target)
	if canonicalBase != "" && canonicalTarget != "" {
		if canonicalRel, canonicalErr := filepath.Rel(canonicalBase, canonicalTarget); canonicalErr == nil {
			return filepath.ToSlash(canonicalRel), true
		}
	}
	return filepath.ToSlash(rel), true
}

func readDiscoverSnippet(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return string(data), nil
}

func isSupportedDiscoverExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".sql", ".xml", ".yaml", ".yml", ".json", ".class", ".jar":
		return true
	default:
		return false
	}
}

func isLiquibaseCandidateExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".xml", ".yaml", ".yml", ".json", ".sql", ".class", ".jar":
		return true
	default:
		return false
	}
}
