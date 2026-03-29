// Package diff provides sqlrs diff: build file lists per kind (psql, lb) in two
// contexts (path or git refs), compare them, and render Added/Modified/Removed.
// See docs/architecture/diff-component-structure.md and docs/user-guides/sqlrs-diff.md.
package diff

// Context is the root from which to read files for one side of the diff.
// BaseDir is the effective working directory for resolving wrapped relative
// paths within that side. In path mode it defaults to Root; in ref worktree mode
// it mirrors the original cwd inside the detached worktree.
// GitRef, when non-empty, selects blob reads at that revision from Root (repo
// toplevel) via inputset.GitRevFileSystem instead of the OS filesystem.
type Context struct {
	Root    string
	BaseDir string
	GitRef  string
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

// ScopeKind selects how the two comparison roots are supplied.
type ScopeKind string

const (
	ScopeKindPath ScopeKind = "path"
	ScopeKindRef  ScopeKind = "ref"
)

// Scope is either path mode (--from-path / --to-path) or ref mode (--from-ref / --to-ref).
type Scope struct {
	Kind ScopeKind

	// Path mode
	FromPath string
	ToPath   string

	// Ref mode: "blob" (default, read objects from Git) or "worktree" (checkout)
	FromRef         string
	ToRef           string
	RefMode         string
	RefKeepWorktree bool
}

// Options holds diff-specific options (limit, include content in output).
type Options struct {
	Limit          int
	IncludeContent bool
}
