//go:build linux

package snapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type btrfsFakeRunner struct {
	calls []runCall
	err   error
}

func (f *btrfsFakeRunner) Run(ctx context.Context, name string, args []string) error {
	f.calls = append(f.calls, runCall{name: name, args: append([]string{}, args...)})
	return f.err
}

type btrfsSequencedRunner struct {
	calls      []runCall
	failOnCall int
	err        error
}

func (r *btrfsSequencedRunner) Run(ctx context.Context, name string, args []string) error {
	r.calls = append(r.calls, runCall{name: name, args: append([]string{}, args...)})
	if r.failOnCall == len(r.calls) {
		return r.err
	}
	return nil
}

func TestBtrfsManagerCloneRequiresDirs(t *testing.T) {
	mgr := btrfsManager{runner: &btrfsFakeRunner{}}
	if _, err := mgr.Clone(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for missing src")
	}
	if _, err := mgr.Clone(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for missing dest")
	}
}

func TestBtrfsManagerCloneCreatesSnapshot(t *testing.T) {
	runner := &btrfsFakeRunner{}
	mgr := btrfsManager{runner: runner}
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destParent := filepath.Join(t.TempDir(), "states")
	dest := filepath.Join(destParent, "state-1")

	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if res.MountDir != dest {
		t.Fatalf("unexpected mount dir: %s", res.MountDir)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one command call, got %+v", runner.calls)
	}
	if runner.calls[0].name != "btrfs" {
		t.Fatalf("expected btrfs command, got %+v", runner.calls[0])
	}
	if want := []string{"subvolume", "snapshot", src, dest}; !equalArgs(runner.calls[0].args, want) {
		t.Fatalf("unexpected args: %+v", runner.calls[0].args)
	}
	if _, err := os.Stat(destParent); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
	if err := res.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if len(runner.calls) != 2 || !equalArgs(runner.calls[1].args, []string{"subvolume", "delete", dest}) {
		t.Fatalf("expected delete call, got %+v", runner.calls)
	}
}

func TestBtrfsManagerCleanupReturnsDeleteError(t *testing.T) {
	runner := &btrfsSequencedRunner{failOnCall: 2, err: errors.New("boom")}
	mgr := btrfsManager{runner: runner}
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "state-1")
	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := res.Cleanup(); err == nil {
		t.Fatalf("expected cleanup error")
	}
}

func TestBtrfsManagerCloneMkdirError(t *testing.T) {
	prevMkdir := osMkdirAllBtrfs
	osMkdirAllBtrfs = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	t.Cleanup(func() { osMkdirAllBtrfs = prevMkdir })

	mgr := btrfsManager{runner: &btrfsFakeRunner{}}
	if _, err := mgr.Clone(context.Background(), "src", filepath.Join(t.TempDir(), "dest", "state")); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestBtrfsManagerCloneCommandError(t *testing.T) {
	mgr := btrfsManager{runner: &btrfsFakeRunner{err: errors.New("boom")}}
	if _, err := mgr.Clone(context.Background(), "src", filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Fatalf("expected command error")
	}
}

func TestBtrfsManagerSnapshotRequiresDirs(t *testing.T) {
	mgr := btrfsManager{runner: &btrfsFakeRunner{}}
	if err := mgr.Snapshot(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for missing src")
	}
	if err := mgr.Snapshot(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for missing dest")
	}
}

func TestBtrfsManagerSnapshotCreatesReadonlySubvolume(t *testing.T) {
	runner := &btrfsFakeRunner{}
	mgr := btrfsManager{runner: runner}
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "snap", "state-1")

	if err := mgr.Snapshot(context.Background(), src, dest); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one command call, got %+v", runner.calls)
	}
	if want := []string{"subvolume", "snapshot", "-r", src, dest}; !equalArgs(runner.calls[0].args, want) {
		t.Fatalf("unexpected args: %+v", runner.calls[0].args)
	}
}

func TestBtrfsManagerSnapshotMkdirError(t *testing.T) {
	prevMkdir := osMkdirAllBtrfs
	osMkdirAllBtrfs = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	t.Cleanup(func() { osMkdirAllBtrfs = prevMkdir })

	mgr := btrfsManager{runner: &btrfsFakeRunner{}}
	if err := mgr.Snapshot(context.Background(), "src", filepath.Join(t.TempDir(), "dest", "state")); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestBtrfsManagerSnapshotCommandError(t *testing.T) {
	mgr := btrfsManager{runner: &btrfsFakeRunner{err: errors.New("boom")}}
	if err := mgr.Snapshot(context.Background(), "src", filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Fatalf("expected command error")
	}
}

func TestBtrfsManagerDestroyCommandError(t *testing.T) {
	mgr := btrfsManager{runner: &btrfsFakeRunner{err: errors.New("boom")}}
	if err := mgr.Destroy(context.Background(), "state"); err == nil {
		t.Fatalf("expected destroy error")
	}
}

func TestBtrfsEnsureSubvolumeCreates(t *testing.T) {
	prevStat := osStatBtrfs
	osStatBtrfs = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { osStatBtrfs = prevStat })

	runner := &btrfsFakeRunner{}
	mgr := btrfsManager{runner: runner}
	path := filepath.Join(t.TempDir(), "state-1")
	if err := mgr.EnsureSubvolume(context.Background(), path); err != nil {
		t.Fatalf("EnsureSubvolume: %v", err)
	}
	if len(runner.calls) != 1 || !equalArgs(runner.calls[0].args, []string{"subvolume", "create", path}) {
		t.Fatalf("unexpected calls: %+v", runner.calls)
	}
}

func TestBtrfsEnsureSubvolumeExistingSubvolume(t *testing.T) {
	prevStat := osStatBtrfs
	osStatBtrfs = func(string) (os.FileInfo, error) {
		return &fakeFileInfo{}, nil
	}
	t.Cleanup(func() { osStatBtrfs = prevStat })

	runner := &btrfsFakeRunner{}
	mgr := btrfsManager{runner: runner}
	path := filepath.Join(t.TempDir(), "state-1")
	if err := mgr.EnsureSubvolume(context.Background(), path); err != nil {
		t.Fatalf("EnsureSubvolume: %v", err)
	}
	if len(runner.calls) != 1 || !equalArgs(runner.calls[0].args, []string{"subvolume", "show", path}) {
		t.Fatalf("unexpected calls: %+v", runner.calls)
	}
}

func TestBtrfsEnsureSubvolumeRejectsNonSubvolume(t *testing.T) {
	prevStat := osStatBtrfs
	osStatBtrfs = func(string) (os.FileInfo, error) {
		return &fakeFileInfo{}, nil
	}
	t.Cleanup(func() { osStatBtrfs = prevStat })

	runner := &btrfsFakeRunner{err: errors.New("not subvolume")}
	mgr := btrfsManager{runner: runner}
	path := filepath.Join(t.TempDir(), "state-1")
	if err := mgr.EnsureSubvolume(context.Background(), path); err == nil {
		t.Fatalf("expected error for non-subvolume")
	}
}

func TestBtrfsEnsureSubvolumeMkdirError(t *testing.T) {
	prevMkdir := osMkdirAllBtrfs
	osMkdirAllBtrfs = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}
	t.Cleanup(func() { osMkdirAllBtrfs = prevMkdir })

	mgr := btrfsManager{runner: &btrfsFakeRunner{}}
	if err := mgr.EnsureSubvolume(context.Background(), "state"); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestBtrfsEnsureSubvolumeCreateError(t *testing.T) {
	prevStat := osStatBtrfs
	osStatBtrfs = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { osStatBtrfs = prevStat })

	mgr := btrfsManager{runner: &btrfsFakeRunner{err: errors.New("boom")}}
	if err := mgr.EnsureSubvolume(context.Background(), "state"); err == nil {
		t.Fatalf("expected create error")
	}
}

func TestBtrfsSupportedUsesStatfs(t *testing.T) {
	prevStatfs := statfsFn
	prevLookPath := execLookPathBtrfs
	t.Cleanup(func() { statfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathBtrfs = prevLookPath })

	var gotPath string
	statfsFn = func(path string, stat *syscall.Statfs_t) error {
		gotPath = path
		stat.Type = btrfsSuperMagic
		return nil
	}
	execLookPathBtrfs = func(string) (string, error) {
		return "/usr/bin/btrfs", nil
	}
	if !btrfsSupported("/data") {
		t.Fatalf("expected btrfs supported")
	}
	if gotPath != "/data" {
		t.Fatalf("expected statfs path /data, got %s", gotPath)
	}
}

func TestBtrfsSupportedRejectsMissingPath(t *testing.T) {
	if btrfsSupported("") {
		t.Fatalf("expected unsupported for empty path")
	}
}

func TestBtrfsSupportedStatfsError(t *testing.T) {
	prevStatfs := statfsFn
	prevLookPath := execLookPathBtrfs
	statfsFn = func(string, *syscall.Statfs_t) error {
		return errors.New("boom")
	}
	execLookPathBtrfs = func(string) (string, error) {
		return "/usr/bin/btrfs", nil
	}
	t.Cleanup(func() { statfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathBtrfs = prevLookPath })

	if btrfsSupported("/data") {
		t.Fatalf("expected unsupported on statfs error")
	}
}

func TestBtrfsSupportedNonBtrfs(t *testing.T) {
	prevStatfs := statfsFn
	prevLookPath := execLookPathBtrfs
	statfsFn = func(path string, stat *syscall.Statfs_t) error {
		stat.Type = 0
		return nil
	}
	execLookPathBtrfs = func(string) (string, error) {
		return "/usr/bin/btrfs", nil
	}
	t.Cleanup(func() { statfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathBtrfs = prevLookPath })

	if btrfsSupported("/data") {
		t.Fatalf("expected unsupported on non-btrfs fs")
	}
}

func TestBtrfsSupportedMissingBinary(t *testing.T) {
	prevLookPath := execLookPathBtrfs
	prevStatfs := statfsFn
	execLookPathBtrfs = func(string) (string, error) {
		return "", errors.New("missing")
	}
	statfsFn = func(string, *syscall.Statfs_t) error {
		t.Fatalf("statfs should not be called when btrfs is missing")
		return nil
	}
	t.Cleanup(func() { execLookPathBtrfs = prevLookPath })
	t.Cleanup(func() { statfsFn = prevStatfs })

	if btrfsSupported("/data") {
		t.Fatalf("expected unsupported without btrfs binary")
	}
}

func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return true }
func (fakeFileInfo) Sys() any           { return nil }
