package diff

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BuildLbFileList builds the ordered file list for plan:lb / prepare:lb:
// entry point from --changelog-file in args, closure over include/includeAll.
func BuildLbFileList(ctx Context, args []string) (FileList, error) {
	changelogPath, err := extractChangelogFileArg(args)
	if err != nil {
		return FileList{}, err
	}
	if changelogPath == "" {
		return FileList{}, fmt.Errorf("liquibase command has no --changelog-file (required for diff)")
	}
	absChangelog := filepath.Join(ctx.Root, filepath.FromSlash(changelogPath))
	tracker := &lbTracker{seen: make(map[string]struct{})}
	var order []string
	if err := tracker.collect(absChangelog, filepath.Dir(absChangelog), &order); err != nil {
		return FileList{}, err
	}
	entries := make([]FileEntry, 0, len(order))
	for _, p := range order {
		content, err := os.ReadFile(p)
		if err != nil {
			return FileList{}, fmt.Errorf("read %s: %w", p, err)
		}
		rel, _ := filepath.Rel(ctx.Root, p)
		entries = append(entries, FileEntry{Path: filepath.ToSlash(rel), Hash: HashContent(content)})
	}
	return FileList{Entries: entries}, nil
}

func extractChangelogFileArg(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--changelog-file" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --changelog-file")
			}
			return strings.TrimSpace(args[i+1]), nil
		}
		if strings.HasPrefix(arg, "--changelog-file=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--changelog-file=")), nil
		}
	}
	return "", nil
}

type lbChangelog struct {
	XMLName   xml.Name       `xml:"databaseChangeLog"`
	Include   []lbInclude   `xml:"include"`
	IncludeAll []lbIncludeAll `xml:"includeAll"`
}

type lbInclude struct {
	File string `xml:"file,attr"`
}

type lbIncludeAll struct {
	Path string `xml:"path,attr"`
}

type lbTracker struct {
	seen map[string]struct{}
}

func (t *lbTracker) collect(absPath, baseDir string, order *[]string) error {
	absPath = filepath.Clean(absPath)
	if _, ok := t.seen[absPath]; ok {
		return nil
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}
	t.seen[absPath] = struct{}{}
	*order = append(*order, absPath)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	var log lbChangelog
	if err := xml.Unmarshal(content, &log); err != nil {
		return nil
	}
	dir := filepath.Dir(absPath)
	for _, inc := range log.Include {
		f := strings.TrimSpace(inc.File)
		if f == "" {
			continue
		}
		if strings.Contains(f, "://") || strings.HasPrefix(strings.ToLower(f), "classpath:") {
			continue
		}
		next := filepath.Join(dir, filepath.FromSlash(f))
		if err := t.collect(next, dir, order); err != nil {
			return err
		}
	}
	for _, inc := range log.IncludeAll {
		pathAttr := strings.TrimSpace(inc.Path)
		if pathAttr == "" {
			continue
		}
		dirPath := filepath.Join(dir, filepath.FromSlash(pathAttr))
		if err := t.collectIncludeAll(dirPath, order); err != nil {
			return err
		}
	}
	return nil
}

func (t *lbTracker) collectIncludeAll(dirPath string, order *[]string) error {
	dirPath = filepath.Clean(dirPath)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("includeAll %s: %w", dirPath, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".sql") ||
			strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
			strings.HasSuffix(lower, ".json") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		abs := filepath.Join(dirPath, name)
		if err := t.collect(abs, dirPath, order); err != nil {
			return err
		}
	}
	return nil
}
