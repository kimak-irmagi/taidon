package diff

import (
	"path/filepath"
)

// Compare builds Added, Modified, Removed from two file lists.
// Paths are normalized (slash) for comparison.
func Compare(from, to FileList, _ Options) DiffResult {
	fromByPath := make(map[string]string)
	for _, e := range from.Entries {
		norm := filepath.ToSlash(e.Path)
		fromByPath[norm] = e.Hash
	}
	toByPath := make(map[string]string)
	for _, e := range to.Entries {
		norm := filepath.ToSlash(e.Path)
		toByPath[norm] = e.Hash
	}
	var added, modified, removed []FileEntry
	for p, hash := range toByPath {
		if fromHash, ok := fromByPath[p]; !ok {
			added = append(added, FileEntry{Path: p, Hash: hash})
		} else if fromHash != hash {
			modified = append(modified, FileEntry{Path: p, Hash: hash})
		}
	}
	for p, hash := range fromByPath {
		if _, ok := toByPath[p]; !ok {
			removed = append(removed, FileEntry{Path: p, Hash: hash})
		}
	}
	return DiffResult{Added: added, Modified: modified, Removed: removed}
}
