package inputset

import (
<<<<<<< Updated upstream
	"fmt"
	"io/fs"
=======
	"bytes"
	"fmt"
	"io/fs"
	"os"
>>>>>>> Stashed changes
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

<<<<<<< Updated upstream
// GitRevFileSystem reads the Git object database at a fixed revision without
// checkout. Absolute paths must lie under RepoRoot; they are mapped to
// ref-relative paths for git show / ls-tree / cat-file.
=======
// GitRevFileSystem reads tree/blob objects from a Git revision without a
// worktree checkout. Paths passed to Stat, ReadFile, and ReadDir are absolute
// host paths that must lie under RepoRoot (same contract as OSFileSystem for
// collectors using WorkspaceResolver).
>>>>>>> Stashed changes
type GitRevFileSystem struct {
	RepoRoot string
	Ref      string
}

<<<<<<< Updated upstream
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
=======
// NewGitRevFileSystem returns a FileSystem backed by `git cat-file` / `git ls-tree`
// at the given revision. RepoRoot must be the repository toplevel; Ref is any
// revision accepted by Git (branch, tag, SHA).
func NewGitRevFileSystem(repoRoot, ref string) GitRevFileSystem {
	return GitRevFileSystem{
		RepoRoot: filepath.Clean(strings.TrimSpace(repoRoot)),
		Ref:      strings.TrimSpace(ref),
	}
}

func (g GitRevFileSystem) absToRel(abs string) (string, error) {
	if g.RepoRoot == "" || g.Ref == "" {
		return "", fmt.Errorf("git rev fs: empty repo root or ref")
	}
	root := CanonicalizeBoundaryPath(g.RepoRoot)
	target := CanonicalizeBoundaryPath(abs)
	if root == "" || target == "" {
		return "", fmt.Errorf("git rev fs: invalid path")
	}
	if !IsWithin(root, target) {
		return "", fmt.Errorf("git rev fs: path outside repo root")
	}
	if target == root {
		return "", nil
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Clean(rel)), nil
}

func (g GitRevFileSystem) objectSpecForRel(rel string) string {
	if rel == "" {
		return g.Ref + "^{tree}"
	}
	return g.Ref + ":" + rel
}

func (g GitRevFileSystem) catFileType(spec string) (string, error) {
	cmd := exec.Command("git", "-C", g.RepoRoot, "cat-file", "-t", spec)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g GitRevFileSystem) catBlobSize(spec string) (int64, error) {
	cmd := exec.Command("git", "-C", g.RepoRoot, "cat-file", "-s", spec)
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
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
=======
// Stat implements FileSystem.
func (g GitRevFileSystem) Stat(path string) (fs.FileInfo, error) {
	rel, err := g.absToRel(path)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: path, Err: err}
	}
	spec := g.objectSpecForRel(rel)
	typ, err := g.catFileType(spec)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	switch typ {
	case "blob":
		sz, err := g.catBlobSize(spec)
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
		}
		name := filepath.Base(path)
		if rel == "" {
			name = "."
		}
		return gitFileInfo{name: name, size: sz, dir: false}, nil
	case "tree":
		name := filepath.Base(path)
		if rel == "" {
			name = filepath.Base(g.RepoRoot)
			if name == "" || name == "." {
				name = "."
			}
		}
		return gitFileInfo{name: name, size: 0, dir: true}, nil
	default:
		return nil, &fs.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
}

// ReadFile implements FileSystem.
func (g GitRevFileSystem) ReadFile(path string) ([]byte, error) {
	rel, err := g.absToRel(path)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		return nil, fmt.Errorf("git rev fs: is a directory")
	}
	spec := g.objectSpecForRel(rel)
	typ, err := g.catFileType(spec)
	if err != nil {
		return nil, err
	}
	if typ != "blob" {
		return nil, fmt.Errorf("git rev fs: not a regular file")
	}
	cmd := exec.Command("git", "-C", g.RepoRoot, "cat-file", "-p", spec)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadDir implements FileSystem.
func (g GitRevFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	rel, err := g.absToRel(path)
	if err != nil {
		return nil, err
	}
	fi, err := g.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: path, Err: fmt.Errorf("not a directory")}
	}
	// Use rev:path so Git lists the tree's children (git ls-tree rev -- path
	// only prints the path's own ls-tree record, not directory contents).
	args := []string{"-C", g.RepoRoot, "ls-tree", "-z"}
	if rel == "" {
		args = append(args, g.objectSpecForRel(""))
	} else {
		args = append(args, g.Ref+":"+rel)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var entries []fs.DirEntry
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		name := string(rec[tab+1:])
		if name == "" {
			continue
		}
		head := strings.Fields(string(rec[:tab]))
		if len(head) < 2 {
			continue
		}
		typ := head[1]
		isDir := typ == "tree"
		entries = append(entries, gitDirEntry{name: name, isDir: isDir})
	}
	return entries, nil
}

type gitFileInfo struct {
	name string
	size int64
	dir  bool
}

func (g gitFileInfo) Name() string       { return g.name }
func (g gitFileInfo) Size() int64        { return g.size }
func (g gitFileInfo) Mode() fs.FileMode  { return 0o644 }
func (g gitFileInfo) ModTime() time.Time { return time.Time{} }
func (g gitFileInfo) IsDir() bool        { return g.dir }
func (g gitFileInfo) Sys() any           { return nil }

type gitDirEntry struct {
	name  string
	isDir bool
}

func (e gitDirEntry) Name() string { return e.name }
func (e gitDirEntry) IsDir() bool  { return e.isDir }
func (e gitDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e gitDirEntry) Info() (fs.FileInfo, error) { return gitFileInfo{name: e.name, dir: e.isDir}, nil }
>>>>>>> Stashed changes
