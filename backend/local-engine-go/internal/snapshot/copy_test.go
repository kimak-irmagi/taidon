package snapshot

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCopyManagerCloneSnapshotDestroy(t *testing.T) {
	src := t.TempDir()
	filePath := filepath.Join(src, "init.sql")
	if err := os.WriteFile(filePath, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "clone")

	manager := CopyManager{}
	clone, err := manager.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clone.MountDir, "init.sql")); err != nil {
		t.Fatalf("expected cloned file: %v", err)
	}
	if err := clone.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected clone dir removed")
	}

	snapshotDir := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.Snapshot(context.Background(), src, snapshotDir); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshotDir, "init.sql")); err != nil {
		t.Fatalf("expected snapshot file: %v", err)
	}
	if err := manager.Destroy(context.Background(), snapshotDir); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func TestNewManagerPrefersCopyWhenOverlayUnavailable(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("overlay availability depends on host config")
	}
	manager := NewManager(Options{PreferOverlay: true})
	if manager.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", manager.Kind())
	}
}

func TestNewManagerWithoutOverlayPreference(t *testing.T) {
	manager := NewManager(Options{PreferOverlay: false})
	if manager.Kind() != "copy" {
		t.Fatalf("expected copy manager, got %s", manager.Kind())
	}
}

func TestCopyDirRejectsMissingSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "missing")
	if err := copyDir(context.Background(), src, filepath.Join(dir, "dest")); err == nil {
		t.Fatalf("expected error for missing source")
	}
}

func TestCopyManagerCloneMissingSource(t *testing.T) {
	manager := CopyManager{}
	if _, err := manager.Clone(context.Background(), "missing", filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Fatalf("expected error for missing source")
	}
}

func TestCopyDirRejectsDestInsideSource(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := copyDir(context.Background(), src, filepath.Join(src, "dest")); err == nil {
		t.Fatalf("expected error for dest inside source")
	}
}

func TestCopyDirRejectsFileSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := copyDir(context.Background(), src, filepath.Join(dir, "dest")); err == nil {
		t.Fatalf("expected error for file source")
	}
}

func TestCopyDirCopiesSymlink(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(src, "target.txt")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(src, "link.txt")
	if err := os.Symlink("target.txt", link); err != nil {
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "privilege") {
			t.Skip("symlink requires privileges on Windows")
		}
		t.Fatalf("symlink: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest")
	if err := copyDir(context.Background(), src, dest); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	copied := filepath.Join(dest, "link.txt")
	info, err := os.Lstat(copied)
	if err != nil {
		t.Fatalf("stat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink, got mode %v", info.Mode())
	}
}

func TestCopyDirRejectsEmptyPaths(t *testing.T) {
	if err := copyDir(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for empty source")
	}
	if err := copyDir(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for empty dest")
	}
}

func TestCopyDirDestIsFile(t *testing.T) {
	src := t.TempDir()
	filePath := filepath.Join(src, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	destRoot := t.TempDir()
	destFile := filepath.Join(destRoot, "dest")
	if err := os.WriteFile(destFile, []byte("y"), 0o600); err != nil {
		t.Fatalf("write dest file: %v", err)
	}
	if err := copyDir(context.Background(), src, destFile); err == nil {
		t.Fatalf("expected error for dest file")
	}
}

func TestCopyDirContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest")

	if err := copyDir(ctx, src, dest); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	if err := copyFile(filepath.Join(t.TempDir(), "missing.txt"), filepath.Join(t.TempDir(), "dest.txt"), 0o600); err == nil {
		t.Fatalf("expected error for missing source")
	}
}

func TestCopyFileRejectsDirectoryDest(t *testing.T) {
	src := t.TempDir()
	filePath := filepath.Join(src, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	destDir := filepath.Join(t.TempDir(), "dest")
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := copyFile(filePath, destDir, 0o600); err == nil {
		t.Fatalf("expected error for directory dest")
	}
}

func TestCopyManagerCapabilities(t *testing.T) {
	caps := (CopyManager{}).Capabilities()
	if !caps.RequiresDBStop {
		t.Fatalf("expected RequiresDBStop true")
	}
	if !caps.SupportsWritableClone {
		t.Fatalf("expected SupportsWritableClone true")
	}
	if caps.SupportsSendReceive {
		t.Fatalf("expected SupportsSendReceive false")
	}
}

func TestCopyDirAbsError(t *testing.T) {
	prev := filepathAbs
	filepathAbs = func(string) (string, error) {
		return "", errors.New("abs boom")
	}
	defer func() { filepathAbs = prev }()

	if err := copyDir(context.Background(), "src", "dest"); err == nil || !strings.Contains(err.Error(), "abs boom") {
		t.Fatalf("expected abs error, got %v", err)
	}
}

func TestCopyDirRelErrorSkipsInsideCheck(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest")

	prevRel := filepathRel
	calls := 0
	filepathRel = func(base, target string) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("rel boom")
		}
		return prevRel(base, target)
	}
	defer func() { filepathRel = prevRel }()

	if err := copyDir(context.Background(), src, dest); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
}

func TestCopyDirWalkError(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		return fn(root, nil, errors.New("walk boom"))
	}
	defer func() { filepathWalkDir = prevWalk }()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "walk boom") {
		t.Fatalf("expected walk error, got %v", err)
	}
}

func TestCopyDirRelErrorDuringWalk(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	prevRel := filepathRel
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{isDir: false, info: fakeFileInfo{mode: 0o600}}
		return fn(filepath.Join(root, "child"), entry, nil)
	}
	filepathRel = func(base, target string) (string, error) {
		return "", errors.New("rel boom")
	}
	defer func() {
		filepathWalkDir = prevWalk
		filepathRel = prevRel
	}()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "rel boom") {
		t.Fatalf("expected rel error, got %v", err)
	}
}

func TestCopyDirEntryInfoErrorDir(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{isDir: true, infoErr: errors.New("info boom")}
		return fn(filepath.Join(root, "child"), entry, nil)
	}
	defer func() { filepathWalkDir = prevWalk }()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "info boom") {
		t.Fatalf("expected info error, got %v", err)
	}
}

func TestCopyDirEntryInfoErrorFile(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{isDir: false, infoErr: errors.New("info boom")}
		return fn(filepath.Join(root, "child"), entry, nil)
	}
	defer func() { filepathWalkDir = prevWalk }()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "info boom") {
		t.Fatalf("expected info error, got %v", err)
	}
}

func TestCopyDirMkdirAllError(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	prevMkdir := osMkdirAll
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{isDir: true, info: fakeFileInfo{mode: 0o700 | os.ModeDir}}
		return fn(filepath.Join(root, "child"), entry, nil)
	}
	osMkdirAll = func(path string, perm os.FileMode) error {
		if strings.Contains(path, "child") {
			return errors.New("mkdir boom")
		}
		return prevMkdir(path, perm)
	}
	defer func() {
		filepathWalkDir = prevWalk
		osMkdirAll = prevMkdir
	}()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "mkdir boom") {
		t.Fatalf("expected mkdir error, got %v", err)
	}
}

func TestCopyDirReadlinkError(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	prevReadlink := osReadlink
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{
			isDir: false,
			info:  fakeFileInfo{mode: os.ModeSymlink},
		}
		return fn(filepath.Join(root, "link"), entry, nil)
	}
	osReadlink = func(string) (string, error) {
		return "", errors.New("readlink boom")
	}
	defer func() {
		filepathWalkDir = prevWalk
		osReadlink = prevReadlink
	}()

	if err := copyDir(context.Background(), src, dest); err == nil || !strings.Contains(err.Error(), "readlink boom") {
		t.Fatalf("expected readlink error, got %v", err)
	}
}

func TestCopyDirSymlinkSuccess(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "dest")

	prevWalk := filepathWalkDir
	prevReadlink := osReadlink
	prevSymlink := osSymlink
	filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
		entry := fakeDirEntry{
			isDir: false,
			info:  fakeFileInfo{mode: os.ModeSymlink},
		}
		return fn(filepath.Join(root, "link"), entry, nil)
	}
	osReadlink = func(string) (string, error) {
		return "target.txt", nil
	}
	linked := ""
	osSymlink = func(oldname, newname string) error {
		linked = oldname + "->" + newname
		return nil
	}
	defer func() {
		filepathWalkDir = prevWalk
		osReadlink = prevReadlink
		osSymlink = prevSymlink
	}()

	if err := copyDir(context.Background(), src, dest); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	if linked == "" {
		t.Fatalf("expected symlink to be created")
	}
}

func TestCopyFileOpenFileError(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "file.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest.txt")

	prevOpenFile := osOpenFile
	osOpenFile = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("openfile boom")
	}
	defer func() { osOpenFile = prevOpenFile }()

	if err := copyFile(srcFile, dest, 0o600); err == nil || !strings.Contains(err.Error(), "openfile boom") {
		t.Fatalf("expected openfile error, got %v", err)
	}
}

func TestCopyFileCopyError(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "file.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest.txt")

	prevCopy := ioCopyFn
	ioCopyFn = func(io.Writer, io.Reader) (int64, error) {
		return 0, errors.New("copy boom")
	}
	defer func() { ioCopyFn = prevCopy }()

	if err := copyFile(srcFile, dest, 0o600); err == nil || !strings.Contains(err.Error(), "copy boom") {
		t.Fatalf("expected copy error, got %v", err)
	}
}

type fakeDirEntry struct {
	name    string
	isDir   bool
	info    os.FileInfo
	infoErr error
}

func (f fakeDirEntry) Name() string {
	if f.name == "" {
		return "entry"
	}
	return f.name
}

func (f fakeDirEntry) IsDir() bool {
	return f.isDir
}

func (f fakeDirEntry) Type() os.FileMode {
	if f.info != nil {
		return f.info.Mode().Type()
	}
	if f.isDir {
		return os.ModeDir
	}
	return 0
}

func (f fakeDirEntry) Info() (os.FileInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	if f.info != nil {
		return f.info, nil
	}
	return fakeFileInfo{mode: 0o600}, nil
}

type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "file" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }
