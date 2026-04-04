// Package inputset provides the shared CLI-side source of truth for file-bearing
// command semantics described in docs/architecture/inputset-component-structure.md.
package inputset

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// UserError marks user-facing input/validation failures so CLI command wrappers
// can preserve their current exit-code behavior while sharing one implementation.
type UserError struct {
	Code    string
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

// Errorf constructs a user-facing validation error owned by the shared layer.
func Errorf(code string, format string, args ...any) *UserError {
	return &UserError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Resolver keeps host-path resolution in one reusable component so execution,
// diff, and alias inspection all use the same workspace-bound rules.
type Resolver struct {
	Root    string
	BaseDir string
	Convert func(string) (string, error)
}

// NewWorkspaceResolver resolves local paths relative to cwd within workspaceRoot.
func NewWorkspaceResolver(workspaceRoot string, cwd string, convert func(string) (string, error)) Resolver {
	root := strings.TrimSpace(workspaceRoot)
	baseDir := strings.TrimSpace(cwd)
	if root == "" {
		root = baseDir
	}
	if baseDir == "" {
		baseDir = root
	}
	return Resolver{
		Root:    filepath.Clean(root),
		BaseDir: filepath.Clean(baseDir),
		Convert: convert,
	}
}

// NewAliasResolver resolves file-bearing alias arguments from the alias file directory.
func NewAliasResolver(workspaceRoot string, aliasPath string) Resolver {
	return NewWorkspaceResolver(workspaceRoot, filepath.Dir(aliasPath), nil)
}

// NewDiffResolver resolves both direct refs and closure edges within one diff side root.
func NewDiffResolver(root string) Resolver {
	return NewWorkspaceResolver(root, root, nil)
}

// ResolvePath resolves a local host path within the configured workspace boundary.
func (r Resolver) ResolvePath(raw string) (string, error) {
	return ResolvePath(raw, r.Root, r.BaseDir, r.Convert)
}

// ResolvePath applies the shared workspace-bound local-path rules.
func ResolvePath(raw string, workspaceRoot string, baseDir string, convert func(string) (string, error)) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", Errorf("empty_path", "file path is empty")
	}

	root := strings.TrimSpace(workspaceRoot)
	base := strings.TrimSpace(baseDir)
	if root == "" {
		root = base
	}
	if base == "" {
		base = root
	}
	root = filepath.Clean(root)
	base = rebasePathToRoot(base, root)

	resolved := cleaned
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(base, filepath.FromSlash(resolved))
	}
	resolved = filepath.Clean(resolved)
	resolved = rebasePathToRoot(resolved, root)

	canonicalRoot := CanonicalizeBoundaryPath(root)
	canonicalResolved := CanonicalizeBoundaryPath(resolved)
	if canonicalRoot != "" && canonicalResolved != "" && !IsWithin(canonicalRoot, canonicalResolved) {
		return "", Errorf("path_outside_workspace", "file path must be within workspace root: %s", resolved)
	}

	if convert != nil {
		converted, err := convert(resolved)
		if err != nil {
			return "", err
		}
		return converted, nil
	}
	return resolved, nil
}

func rebasePathToRoot(path string, root string) string {
	rawPath := strings.TrimSpace(path)
	rawRoot := strings.TrimSpace(root)
	if rawPath == "" || rawRoot == "" {
		return rawPath
	}

	cleanedPath := filepath.Clean(rawPath)
	cleanedRoot := filepath.Clean(rawRoot)
	canonicalRoot := CanonicalizeBoundaryPath(cleanedRoot)
	canonicalPath := CanonicalizeBoundaryPath(cleanedPath)
	if canonicalRoot == "" || canonicalPath == "" || !IsWithin(canonicalRoot, canonicalPath) {
		return cleanedPath
	}

	rel, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return cleanedPath
	}
	if rel == "." {
		return cleanedRoot
	}
	return filepath.Clean(filepath.Join(cleanedRoot, rel))
}

// CanonicalizeBoundaryPath resolves the existing parent path when possible so
// workspace-bound checks stay stable across symlinks and missing trailing paths.
func CanonicalizeBoundaryPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return cleaned
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}

	probe := cleaned
	suffix := make([]string, 0, 4)
	for {
		parent := filepath.Dir(probe)
		if parent == probe {
			return cleaned
		}
		suffix = append([]string{filepath.Base(probe)}, suffix...)
		probe = parent
		if resolved, err := filepath.EvalSymlinks(probe); err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Join(parts...)
		}
	}
}

// IsWithin reports whether target stays within base according to filepath.Rel.
func IsWithin(base string, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

// LooksLikeLiquibaseRemoteRef reports whether Liquibase should treat the value
// as a remote/classpath reference rather than a local host path.
func LooksLikeLiquibaseRemoteRef(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "://") || strings.HasPrefix(lower, "classpath:")
}

// SplitPgbenchFileArgValue keeps the optional @weight suffix attached to a pgbench script ref.
func SplitPgbenchFileArgValue(value string) (string, string) {
	idx := strings.LastIndex(value, "@")
	if idx <= 0 || idx >= len(value)-1 {
		return value, ""
	}
	for _, r := range value[idx+1:] {
		if r < '0' || r > '9' {
			return value, ""
		}
	}
	return value[:idx], value[idx:]
}

// DeclaredRef is a direct path-bearing input declared on the command line.
type DeclaredRef struct {
	Code        string
	Path        string
	RequireFile bool
}

// InputEntry is one deterministic collected file entry.
type InputEntry struct {
	Path    string
	AbsPath string
	Hash    string
}

// InputSet is the collected deterministic file set for one consumer.
type InputSet struct {
	Entries []InputEntry
}

// RunStep is the shared run-step projection used by `run:*` wrappers.
type RunStep struct {
	Args  []string
	Stdin *string
}

// FileSystem isolates filesystem access so collectors stay testable.
type FileSystem interface {
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
}

// BlobOIDer is implemented by GitRevFileSystem. Diff-facing Collect uses Git's
// blob object id (40 lowercase hex chars, SHA-1 of the stored blob) as the
// entry Hash instead of HashContent's SHA-256 over raw file bytes.
type BlobOIDer interface {
	BlobOID(absPath string) (string, error)
}

// OSFileSystem binds the shared layer to the real host filesystem.
type OSFileSystem struct{}

func (OSFileSystem) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

// HashContent returns the deterministic SHA-256 content hash used by diff-facing projections.
func HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
