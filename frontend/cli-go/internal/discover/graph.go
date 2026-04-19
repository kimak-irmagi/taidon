package discover

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/sqlrs/cli/internal/pathutil"
)

// stableDiscoverAbsPath normalizes absolute workspace paths for alias coverage
// lookups so symlinked forms of the same file compare equal.
func stableDiscoverAbsPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(cleaned) {
		return cleaned
	}
	return filepath.Clean(pathutil.CanonicalizeBoundaryPath(cleaned))
}

func discoverPathKeys(workspaceRoot string, relPath string, absPath string) []string {
	keys := make([]string, 0, 2)
	add := func(value string) {
		cleaned := filepath.ToSlash(strings.TrimSpace(value))
		if cleaned == "" {
			return
		}
		if slices.Contains(keys, cleaned) {
			return
		}
		keys = append(keys, cleaned)
	}

	add(relPath)
	if workspaceRoot != "" && filepath.IsAbs(absPath) {
		if rel, err := filepath.Rel(workspaceRoot, absPath); err == nil {
			add(rel)
		}
	}
	add(stableDiscoverAbsPath(absPath))
	return keys
}

func containsAnyKey(set map[string]struct{}, keys []string) bool {
	for _, key := range keys {
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func keySet(keys []string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}

func hasAnyInbound(inbound map[string]int, keys []string) bool {
	for _, key := range keys {
		if inbound[key] > 0 {
			return true
		}
	}
	return false
}

func inboundEdges(candidates []validatedCandidate) map[string]int {
	inbound := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		candidateKeys := discoverPathKeys(candidate.WorkspaceRoot, candidate.WorkspaceRel, candidate.AbsPath)
		if len(candidate.Closure) == 0 {
			continue
		}
		for _, other := range candidates {
			otherKeys := discoverPathKeys(other.WorkspaceRoot, other.WorkspaceRel, other.AbsPath)
			if containsAnyKey(keySet(otherKeys), candidateKeys) {
				continue
			}
			if containsAnyKey(other.Closure, candidateKeys) {
				for _, key := range candidateKeys {
					inbound[key]++
				}
			}
		}
	}
	return inbound
}
