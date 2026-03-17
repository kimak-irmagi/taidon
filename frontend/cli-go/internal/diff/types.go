// Package diff provides path-mode comparison for sqlrs diff: build file lists
// per kind (psql, lb), compare them, and render Added/Modified/Removed.
// See docs/architecture/diff-component-structure.md and docs/user-guides/sqlrs-diff.md.
package diff

// Context is the root from which to read files for one side of the diff.
// In path mode it is the absolute path to a directory.
type Context struct {
	Root string
}

// FileEntry is one file in the ordered list: path relative to context root and content hash.
type FileEntry struct {
	Path string
	Hash string
}

// FileList is the deterministic ordered list of file inputs for one side.
type FileList struct {
	Entries []FileEntry
}

// DiffResult is the result of comparing two file lists.
type DiffResult struct {
	Added    []FileEntry
	Modified []FileEntry // path present on both, hash differs
	Removed  []FileEntry
}

// Scope is the comparison scope. PathScope is the first slice (--from-path / --to-path).
type PathScope struct {
	FromPath string
	ToPath   string
}

// Options holds diff-specific options (limit, include content in output).
type Options struct {
	Limit         int
	IncludeContent bool
}
