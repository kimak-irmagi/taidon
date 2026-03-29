package inputset

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GitRevFileSystem reads the Git object database at a fixed revision without
// checkout. Absolute paths must lie under RepoRoot; they are mapped to
// ref-relative paths for git show / ls-tree / cat-file.
type GitRevFileSystem struct {
	RepoRoot string
	Ref      string
}

// NewGitRevFileSystem returns a FileSystem backed by git objects at Ref.
func NewGitRevFileSystem(repoRoot, ref string) *GitRevFileSystem {
	repoRoot = strings.TrimSpace(repoRoot)
	ref = strings.TrimSpace(ref)
	return &GitRevFileSystem{
		RepoRoot: CanonicalizeBoundaryPath(filepath.Clean(repoRoot)),
		Ref:      ref,
	}
}

func (g *GitRevFileSystem) absToRel(abs string) (string, error) {
	if g.RepoRoot == "" || g.Ref == "" {
		return "", fmt.Errorf("git fs: empty repo or ref")
	}
	abs = CanonicalizeBoundaryPath(filepath.Clean(abs))
	rel, err := filepath.Rel(g.RepoRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside repository root")
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}

func normalizeGitRel(rel string) string {
	rel = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
	rel = strings.TrimPrefix(rel, "./")
	if rel == ".." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return ""
	}
	return rel
}

// ReadFile reads a blob at ref:path (path relative to repo root, slash-separated).
func (g *GitRevFileSystem) ReadFile(abs string) ([]byte, error) {
	rel, err := g.absToRel(abs)
	if err != nil {
		return nil, err
	}
	rel = normalizeGitRel(rel)
	if rel == "" {
		return nil, fmt.Errorf("invalid or empty object path")
	}
	spec := g.Ref + ":" + rel
	cmd := exec.Command("git", "-C", g.RepoRoot, "show", spec)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git show %s: %w: %s", spec, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git show %s: %w", spec, err)
	}
	return out, nil
}

// Stat reports a blob or tree at ref:path. The repository root directory is a tree.
func (g *GitRevFileSystem) Stat(abs string) (fs.FileInfo, error) {
	if g.RepoRoot == "" || g.Ref == "" {
		return nil, fs.ErrInvalid
	}
	canonicalAbs := CanonicalizeBoundaryPath(filepath.Clean(abs))
	if canonicalAbs == g.RepoRoot {
		if err := g.verifyRef(); err != nil {
			return nil, err
		}
		return &gitFileInfo{name: filepath.Base(g.RepoRoot), isDir: true}, nil
	}
	rel, err := g.absToRel(abs)
	if err != nil {
		return nil, err
	}
	rel = normalizeGitRel(rel)
	if rel == "" {
		return nil, &fs.PathError{Op: "stat", Path: abs, Err: fs.ErrNotExist}
	}
	spec := g.Ref + ":" + rel
	cmd := exec.Command("git", "-C", g.RepoRoot, "cat-file", "-t", spec)
	out, err := cmd.Output()
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: abs, Err: fs.ErrNotExist}
	}
	typ := strings.TrimSpace(string(out))
	switch typ {
	case "blob":
		size, _ := gitCatFileSize(g.RepoRoot, spec)
		return &gitFileInfo{name: filepath.Base(abs), size: size}, nil
	case "tree":
		return &gitFileInfo{name: filepath.Base(abs), isDir: true}, nil
	default:
		return nil, &fs.PathError{Op: "stat", Path: abs, Err: fs.ErrNotExist}
	}
}

func (g *GitRevFileSystem) verifyRef() error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "rev-parse", "--verify", g.Ref+"^{commit}")
	if err := cmd.Run(); err != nil {
		return &fs.PathError{Op: "stat", Path: g.RepoRoot, Err: fs.ErrNotExist}
	}
	return nil
}

func gitCatFileSize(repo, spec string) (int64, error) {
	cmd := exec.Command("git", "-C", repo, "cat-file", "-s", spec)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ReadDir lists one level under ref[:rel] (only blobs and trees as immediate children).
func (g *GitRevFileSystem) ReadDir(abs string) ([]fs.DirEntry, error) {
	rel, err := g.absToRel(abs)
	if err != nil {
		return nil, err
	}
	rel = normalizeGitRel(rel)
	treeArg := g.Ref
	if rel != "" {
		treeArg = g.Ref + ":" + rel
	}
	cmd := exec.Command("git", "-C", g.RepoRoot, "ls-tree", "-z", treeArg)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-tree %s: %w: %s", treeArg, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git ls-tree %s: %w", treeArg, err)
	}
	return parseLsTreeDirEntries(out), nil
}

func parseLsTreeDirEntries(out []byte) []fs.DirEntry {
	s := string(out)
	recs := strings.Split(strings.TrimSuffix(s, "\x00"), "\x00")
	var entries []fs.DirEntry
	for _, rec := range recs {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		meta, name := rec[:tab], rec[tab+1:]
		if strings.Contains(name, "/") {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) < 2 {
			continue
		}
		typ := parts[1]
		isDir := typ == "tree"
		if typ != "blob" && typ != "tree" {
			continue
		}
		info := &gitFileInfo{name: name, isDir: isDir}
		entries = append(entries, gitDirEntry{info: info})
	}
	return entries
}

type gitFileInfo struct {
	name  string
	isDir bool
	size  int64
}

func (i *gitFileInfo) Name() string               { return i.name }
func (i *gitFileInfo) Size() int64                { return i.size }
func (i *gitFileInfo) Mode() fs.FileMode          { return i.fileMode() }
func (i *gitFileInfo) ModTime() time.Time         { return time.Time{} }
func (i *gitFileInfo) IsDir() bool                { return i.isDir }
func (i *gitFileInfo) Sys() any                   { return nil }
func (i *gitFileInfo) fileMode() fs.FileMode {
	if i.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

type gitDirEntry struct {
	info *gitFileInfo
}

func (e gitDirEntry) Name() string               { return e.info.Name() }
func (e gitDirEntry) IsDir() bool               { return e.info.IsDir() }
func (e gitDirEntry) Type() fs.FileMode         { return e.info.Mode().Type() }
func (e gitDirEntry) Info() (fs.FileInfo, error) { return e.info, nil }
