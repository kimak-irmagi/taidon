package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RenderHuman writes a human-readable diff to w. If opts.Limit > 0, only that many
// entries per section are listed; summary counts are always full.
// When opts.IncludeContent is true, fromCtx and toCtx supply roots to read file snippets:
// added and modified show content from toCtx; removed from fromCtx.
func RenderHuman(w io.Writer, result DiffResult, scope PathScope, wrappedCommand string, opts Options, fromCtx, toCtx Context) {
	fmt.Fprintf(w, "diff --from-path %s --to-path %s %s\n\n", scope.FromPath, scope.ToPath, wrappedCommand)
	added := result.Added
	modified := result.Modified
	removed := result.Removed
	if opts.Limit > 0 {
		if len(added) > opts.Limit {
			added = added[:opts.Limit]
		}
		if len(modified) > opts.Limit {
			modified = modified[:opts.Limit]
		}
		if len(removed) > opts.Limit {
			removed = removed[:opts.Limit]
		}
	}
	if len(result.Added) > 0 {
		fmt.Fprintln(w, "Added:")
		for _, e := range added {
			fmt.Fprintf(w, "  %s\n", e.Path)
			if opts.IncludeContent {
				snip := strings.TrimSuffix(fileSnippet(toCtx.Root, e.Path), "\n")
				for _, line := range strings.Split(snip, "\n") {
					fmt.Fprintf(w, "    │ %s\n", line)
				}
			}
		}
		if opts.Limit > 0 && len(result.Added) > opts.Limit {
			fmt.Fprintf(w, "  ... (%d more)\n", len(result.Added)-opts.Limit)
		}
		fmt.Fprintln(w)
	}
	if len(result.Modified) > 0 {
		fmt.Fprintln(w, "Modified:")
		for _, e := range modified {
			fmt.Fprintf(w, "  %s\n", e.Path)
			if opts.IncludeContent {
				snip := strings.TrimSuffix(fileSnippet(toCtx.Root, e.Path), "\n")
				for _, line := range strings.Split(snip, "\n") {
					fmt.Fprintf(w, "    │ %s\n", line)
				}
			}
		}
		if opts.Limit > 0 && len(result.Modified) > opts.Limit {
			fmt.Fprintf(w, "  ... (%d more)\n", len(result.Modified)-opts.Limit)
		}
		fmt.Fprintln(w)
	}
	if len(result.Removed) > 0 {
		fmt.Fprintln(w, "Removed:")
		for _, e := range removed {
			fmt.Fprintf(w, "  %s\n", e.Path)
			if opts.IncludeContent {
				snip := strings.TrimSuffix(fileSnippet(fromCtx.Root, e.Path), "\n")
				for _, line := range strings.Split(snip, "\n") {
					fmt.Fprintf(w, "    │ %s\n", line)
				}
			}
		}
		if opts.Limit > 0 && len(result.Removed) > opts.Limit {
			fmt.Fprintf(w, "  ... (%d more)\n", len(result.Removed)-opts.Limit)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Summary: %d added, %d modified, %d removed\n",
		len(result.Added), len(result.Modified), len(result.Removed))
}

// JSONOutput is the stable JSON shape for diff output.
type JSONOutput struct {
	Scope    JSONScope    `json:"scope"`
	Command  string       `json:"command"`
	Added    []JSONEntry  `json:"added"`
	Modified []JSONEntry  `json:"modified"`
	Removed  []JSONEntry  `json:"removed"`
	Summary  JSONSummary  `json:"summary"`
}

type JSONScope struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

type JSONEntry struct {
	Path    string `json:"path"`
	Hash    string `json:"hash,omitempty"`
	Content string `json:"content,omitempty"`
}

type JSONSummary struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Removed  int `json:"removed"`
}

// RenderJSON writes the diff as a single JSON object to w. If opts.Limit > 0,
// added/modified/removed arrays are truncated; summary counts are always full.
// When opts.IncludeContent is true, each entry includes a truncated content snippet
// from the same roots as RenderHuman (to for added/modified, from for removed).
func RenderJSON(w io.Writer, result DiffResult, scope PathScope, wrappedCommand string, opts Options, fromCtx, toCtx Context) error {
	added := result.Added
	modified := result.Modified
	removed := result.Removed
	if opts.Limit > 0 {
		if len(added) > opts.Limit {
			added = added[:opts.Limit]
		}
		if len(modified) > opts.Limit {
			modified = modified[:opts.Limit]
		}
		if len(removed) > opts.Limit {
			removed = removed[:opts.Limit]
		}
	}
	addedJ := make([]JSONEntry, len(added))
	for i, e := range added {
		ent := JSONEntry{Path: e.Path, Hash: e.Hash}
		if opts.IncludeContent {
			ent.Content = fileSnippet(toCtx.Root, e.Path)
		}
		addedJ[i] = ent
	}
	modifiedJ := make([]JSONEntry, len(modified))
	for i, e := range modified {
		ent := JSONEntry{Path: e.Path, Hash: e.Hash}
		if opts.IncludeContent {
			ent.Content = fileSnippet(toCtx.Root, e.Path)
		}
		modifiedJ[i] = ent
	}
	removedJ := make([]JSONEntry, len(removed))
	for i, e := range removed {
		ent := JSONEntry{Path: e.Path, Hash: e.Hash}
		if opts.IncludeContent {
			ent.Content = fileSnippet(fromCtx.Root, e.Path)
		}
		removedJ[i] = ent
	}
	out := JSONOutput{
		Scope:   JSONScope{FromPath: scope.FromPath, ToPath: scope.ToPath},
		Command: wrappedCommand,
		Added:   addedJ,
		Modified: modifiedJ,
		Removed:  removedJ,
		Summary: JSONSummary{
			Added:    len(result.Added),
			Modified: len(result.Modified),
			Removed:  len(result.Removed),
		},
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}
