//go:build linux

package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// zfsFakeRunner records all command invocations and optionally returns an error.
type zfsFakeRunner struct {
	calls []runCall
	err   error
}

func (f *zfsFakeRunner) Run(_ context.Context, name string, args []string) error {
	f.calls = append(f.calls, runCall{name: name, args: append([]string{}, args...)})
	return f.err
}

// zfsSequencedRunner fails on a specific call number.
type zfsSequencedRunner struct {
	calls      []runCall
	failOnCall int
	err        error
}

func (r *zfsSequencedRunner) Run(_ context.Context, name string, args []string) error {
	r.calls = append(r.calls, runCall{name: name, args: append([]string{}, args...)})
	if r.failOnCall == len(r.calls) {
		return r.err
	}
	return nil
}

// withZfsDatasetMap installs a zfsDatasetForPathFn that resolves paths from a
// static map and restores the original on test cleanup.
func withZfsDatasetMap(t *testing.T, m map[string]string) {
	t.Helper()
	prev := zfsDatasetForPathFn
	zfsDatasetForPathFn = func(path string) (string, error) {
		if ds, ok := m[path]; ok {
			return ds, nil
		}
		return "", fmt.Errorf("zfs test: no dataset for path: %s", path)
	}
	t.Cleanup(func() { zfsDatasetForPathFn = prev })
}

func withZfsSnapSuffix(t *testing.T, suffix string) {
	t.Helper()
	prev := zfsNewSnapSuffix
	zfsNewSnapSuffix = func() string { return suffix }
	t.Cleanup(func() { zfsNewSnapSuffix = prev })
}

// ---- Kind & Capabilities -----------------------------------------------

func TestNewZfsManagerKindAndCapabilities(t *testing.T) {
	mgr := newZfsManager()
	if mgr.Kind() != "zfs" {
		t.Fatalf("expected zfs kind, got %s", mgr.Kind())
	}
	caps := mgr.Capabilities()
	if !caps.RequiresDBStop || !caps.SupportsWritableClone || caps.SupportsSendReceive {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}

// ---- Clone ---------------------------------------------------------------

func TestZfsManagerCloneRequiresDirs(t *testing.T) {
	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if _, err := mgr.Clone(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for missing src")
	}
	if _, err := mgr.Clone(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for missing dest")
	}
}

func TestZfsManagerCloneCreatesSnapshotAndClone(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "state-1")

	withZfsSnapSuffix(t, "42")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}

	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if res.MountDir != dest {
		t.Fatalf("unexpected mount dir: %s", res.MountDir)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	snapCall := runner.calls[0]
	if snapCall.name != "zfs" || !equalArgs(snapCall.args, []string{"snapshot", "pool/states/base@taidon-clone-42"}) {
		t.Fatalf("unexpected snapshot call: %+v", snapCall)
	}

	cloneCall := runner.calls[1]
	if cloneCall.name != "zfs" {
		t.Fatalf("expected zfs command, got %s", cloneCall.name)
	}
	wantClone := []string{"clone", "-o", "mountpoint=" + dest, "pool/states/base@taidon-clone-42", "pool/states/state-1"}
	if !equalArgs(cloneCall.args, wantClone) {
		t.Fatalf("unexpected clone args: %+v", cloneCall.args)
	}
}

func TestZfsManagerCloneCleanupDestroysCloneAndSnap(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "state-1")

	withZfsSnapSuffix(t, "99")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}

	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if err := res.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if len(runner.calls) != 4 {
		t.Fatalf("expected 4 calls (snapshot+clone+destroy-clone+destroy-snap), got %d: %+v", len(runner.calls), runner.calls)
	}

	destroyClone := runner.calls[2]
	if !equalArgs(destroyClone.args, []string{"destroy", "pool/states/state-1"}) {
		t.Fatalf("unexpected destroy clone call: %+v", destroyClone)
	}

	destroySnap := runner.calls[3]
	if !equalArgs(destroySnap.args, []string{"destroy", "pool/states/base@taidon-clone-99"}) {
		t.Fatalf("unexpected destroy snap call: %+v", destroySnap)
	}
}

func TestZfsManagerCloneCleanupErrorPropagates(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "state-1")

	withZfsSnapSuffix(t, "1")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	runner := &zfsSequencedRunner{failOnCall: 3, err: errors.New("destroy failed")}
	mgr := zfsManager{runner: runner}

	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := res.Cleanup(); err == nil {
		t.Fatalf("expected cleanup error")
	}
}

func TestZfsManagerCloneSnapshotCommandError(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "state-1")

	withZfsSnapSuffix(t, "1")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	mgr := zfsManager{runner: &zfsFakeRunner{err: errors.New("snapshot failed")}}
	if _, err := mgr.Clone(context.Background(), src, dest); err == nil {
		t.Fatalf("expected snapshot error")
	}
}

func TestZfsManagerCloneCommandError(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "state-1")

	withZfsSnapSuffix(t, "1")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	runner := &zfsSequencedRunner{failOnCall: 2, err: errors.New("clone failed")}
	mgr := zfsManager{runner: runner}
	if _, err := mgr.Clone(context.Background(), src, dest); err == nil {
		t.Fatalf("expected clone error")
	}
}

func TestZfsManagerCloneSrcDatasetError(t *testing.T) {
	prev := zfsDatasetForPathFn
	zfsDatasetForPathFn = func(string) (string, error) {
		return "", errors.New("no dataset")
	}
	t.Cleanup(func() { zfsDatasetForPathFn = prev })

	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if _, err := mgr.Clone(context.Background(), "src", "dest"); err == nil {
		t.Fatalf("expected dataset resolution error")
	}
}

// ---- Snapshot ------------------------------------------------------------

func TestZfsManagerSnapshotRequiresDirs(t *testing.T) {
	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if err := mgr.Snapshot(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for missing src")
	}
	if err := mgr.Snapshot(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for missing dest")
	}
}

func TestZfsManagerSnapshotCreatesReadonlyClone(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "snap-1")

	withZfsSnapSuffix(t, "77")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}

	if err := mgr.Snapshot(context.Background(), src, dest); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(runner.calls), runner.calls)
	}

	snapCall := runner.calls[0]
	if !equalArgs(snapCall.args, []string{"snapshot", "pool/states/base@taidon-snap-77"}) {
		t.Fatalf("unexpected snapshot args: %+v", snapCall.args)
	}

	cloneCall := runner.calls[1]
	wantClone := []string{"clone", "-o", "mountpoint=" + dest, "-o", "readonly=on", "pool/states/base@taidon-snap-77", "pool/states/snap-1"}
	if !equalArgs(cloneCall.args, wantClone) {
		t.Fatalf("unexpected clone args: %+v", cloneCall.args)
	}
}

func TestZfsManagerSnapshotMkdirError(t *testing.T) {
	prev := osMkdirAllZfs
	osMkdirAllZfs = func(string, os.FileMode) error { return errors.New("mkdir failed") }
	t.Cleanup(func() { osMkdirAllZfs = prev })

	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if err := mgr.Snapshot(context.Background(), "src", filepath.Join(t.TempDir(), "dir", "state")); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestZfsManagerSnapshotCommandError(t *testing.T) {
	src := t.TempDir()
	destParent := t.TempDir()
	dest := filepath.Join(destParent, "snap-1")

	withZfsSnapSuffix(t, "1")
	withZfsDatasetMap(t, map[string]string{
		src:        "pool/states/base",
		destParent: "pool/states",
	})

	mgr := zfsManager{runner: &zfsFakeRunner{err: errors.New("boom")}}
	if err := mgr.Snapshot(context.Background(), src, dest); err == nil {
		t.Fatalf("expected error")
	}
}

// ---- Destroy -------------------------------------------------------------

func TestZfsManagerDestroyCallsZfsDestroy(t *testing.T) {
	dir := t.TempDir()

	withZfsDatasetMap(t, map[string]string{dir: "pool/states/state-1"})

	prevOrigin := zfsGetOriginFn
	zfsGetOriginFn = func(context.Context, string) (string, error) { return "-", nil }
	t.Cleanup(func() { zfsGetOriginFn = prevOrigin })

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}

	if err := mgr.Destroy(context.Background(), dir); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if len(runner.calls) != 1 || !equalArgs(runner.calls[0].args, []string{"destroy", "pool/states/state-1"}) {
		t.Fatalf("unexpected calls: %+v", runner.calls)
	}
}

func TestZfsManagerDestroyDestroysOriginSnapshot(t *testing.T) {
	dir := t.TempDir()

	withZfsDatasetMap(t, map[string]string{dir: "pool/states/state-1"})

	prevOrigin := zfsGetOriginFn
	zfsGetOriginFn = func(context.Context, string) (string, error) {
		return "pool/states/base@taidon-clone-99", nil
	}
	t.Cleanup(func() { zfsGetOriginFn = prevOrigin })

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}

	if err := mgr.Destroy(context.Background(), dir); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls (destroy + destroy origin), got %d: %+v", len(runner.calls), runner.calls)
	}
	if !equalArgs(runner.calls[1].args, []string{"destroy", "pool/states/base@taidon-clone-99"}) {
		t.Fatalf("unexpected origin destroy call: %+v", runner.calls[1])
	}
}

func TestZfsManagerDestroyCommandError(t *testing.T) {
	dir := t.TempDir()

	withZfsDatasetMap(t, map[string]string{dir: "pool/states/state-1"})

	prevOrigin := zfsGetOriginFn
	zfsGetOriginFn = func(context.Context, string) (string, error) { return "-", nil }
	t.Cleanup(func() { zfsGetOriginFn = prevOrigin })

	prevList := zfsListDatasetsFn
	zfsListDatasetsFn = func(context.Context, string) (string, error) { return "", nil }
	t.Cleanup(func() { zfsListDatasetsFn = prevList })

	mgr := zfsManager{runner: &zfsFakeRunner{err: errors.New("boom")}}
	if err := mgr.Destroy(context.Background(), dir); err == nil {
		t.Fatalf("expected destroy error")
	}
}

func TestZfsManagerDestroyIncludesNestedListOnError(t *testing.T) {
	dir := t.TempDir()

	withZfsDatasetMap(t, map[string]string{dir: "pool/states/state-1"})

	prevOrigin := zfsGetOriginFn
	zfsGetOriginFn = func(context.Context, string) (string, error) { return "-", nil }
	t.Cleanup(func() { zfsGetOriginFn = prevOrigin })

	prevList := zfsListDatasetsFn
	zfsListDatasetsFn = func(context.Context, string) (string, error) {
		return "pool/states/state-1/child", nil
	}
	t.Cleanup(func() { zfsListDatasetsFn = prevList })

	mgr := zfsManager{runner: &zfsFakeRunner{err: errors.New("boom")}}
	err := mgr.Destroy(context.Background(), dir)
	if err == nil {
		t.Fatalf("expected destroy error")
	}
	if !strings.Contains(err.Error(), "nested datasets") {
		t.Fatalf("expected nested datasets in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "pool/states/state-1/child") {
		t.Fatalf("expected child dataset in error, got %v", err)
	}
}

func TestZfsManagerDestroyDatasetResolutionError(t *testing.T) {
	prev := zfsDatasetForPathFn
	zfsDatasetForPathFn = func(string) (string, error) { return "", errors.New("no dataset") }
	t.Cleanup(func() { zfsDatasetForPathFn = prev })

	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if err := mgr.Destroy(context.Background(), "dir"); err == nil {
		t.Fatalf("expected error when dataset resolution fails")
	}
}

// ---- EnsureDataset -------------------------------------------------------

func TestZfsEnsureDatasetCreates(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state-1")

	withZfsDatasetMap(t, map[string]string{parent: "pool/states"})

	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { osStatZfs = prev })

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}
	if err := mgr.EnsureDataset(context.Background(), path); err != nil {
		t.Fatalf("EnsureDataset: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %+v", len(runner.calls), runner.calls)
	}
	want := []string{"create", "-o", "mountpoint=" + path, "pool/states/state-1"}
	if !equalArgs(runner.calls[0].args, want) {
		t.Fatalf("unexpected args: %+v", runner.calls[0].args)
	}
}

func TestZfsEnsureDatasetExistingDataset(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state-1")

	withZfsDatasetMap(t, map[string]string{parent: "pool/states"})

	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return &zfsFakeFileInfo{}, nil }
	t.Cleanup(func() { osStatZfs = prev })

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}
	if err := mgr.EnsureDataset(context.Background(), path); err != nil {
		t.Fatalf("EnsureDataset for existing dataset: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0].args[0] != "list" {
		t.Fatalf("expected list call, got %+v", runner.calls)
	}
}

func TestZfsEnsureDatasetPathExistsNotDataset(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state-1")

	withZfsDatasetMap(t, map[string]string{parent: "pool/states"})

	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return &zfsFakeFileInfo{}, nil }
	t.Cleanup(func() { osStatZfs = prev })

	runner := &zfsFakeRunner{err: errors.New("not a dataset")}
	mgr := zfsManager{runner: runner}
	if err := mgr.EnsureDataset(context.Background(), path); err == nil {
		t.Fatalf("expected error for non-dataset path")
	}
}

func TestZfsEnsureDatasetMkdirError(t *testing.T) {
	prev := osMkdirAllZfs
	osMkdirAllZfs = func(string, os.FileMode) error { return errors.New("mkdir failed") }
	t.Cleanup(func() { osMkdirAllZfs = prev })

	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if err := mgr.EnsureDataset(context.Background(), "path"); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestZfsEnsureDatasetRequiresPath(t *testing.T) {
	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if err := mgr.EnsureDataset(context.Background(), "  "); err == nil {
		t.Fatalf("expected error for blank path")
	}
}

// ---- IsDataset -----------------------------------------------------------

func TestZfsIsDatasetRequiresPath(t *testing.T) {
	mgr := zfsManager{runner: &zfsFakeRunner{}}
	if _, err := mgr.IsDataset(context.Background(), " "); err == nil {
		t.Fatalf("expected error")
	}
}

func TestZfsIsDatasetNotExists(t *testing.T) {
	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { osStatZfs = prev })

	mgr := zfsManager{runner: &zfsFakeRunner{}}
	ok, err := mgr.IsDataset(context.Background(), "/data/subvol")
	if err != nil || ok {
		t.Fatalf("expected missing to be false, got ok=%v err=%v", ok, err)
	}
}

func TestZfsIsDatasetListErrorReturnsFalse(t *testing.T) {
	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return &zfsFakeFileInfo{}, nil }
	t.Cleanup(func() { osStatZfs = prev })

	prevDS := zfsDatasetForPathFn
	zfsDatasetForPathFn = func(string) (string, error) { return "pool/x", nil }
	t.Cleanup(func() { zfsDatasetForPathFn = prevDS })

	mgr := zfsManager{runner: &zfsFakeRunner{err: errors.New("boom")}}
	ok, err := mgr.IsDataset(context.Background(), "/data/subvol")
	if err != nil || ok {
		t.Fatalf("expected false on list error, got ok=%v err=%v", ok, err)
	}
}

func TestZfsIsDatasetSuccess(t *testing.T) {
	prev := osStatZfs
	osStatZfs = func(string) (os.FileInfo, error) { return &zfsFakeFileInfo{}, nil }
	t.Cleanup(func() { osStatZfs = prev })

	prevDS := zfsDatasetForPathFn
	zfsDatasetForPathFn = func(string) (string, error) { return "pool/states/base", nil }
	t.Cleanup(func() { zfsDatasetForPathFn = prevDS })

	runner := &zfsFakeRunner{}
	mgr := zfsManager{runner: runner}
	ok, err := mgr.IsDataset(context.Background(), "/data/states/base")
	if err != nil || !ok {
		t.Fatalf("expected true, got ok=%v err=%v", ok, err)
	}
	if len(runner.calls) == 0 || runner.calls[0].args[0] != "list" {
		t.Fatalf("expected list call, got %+v", runner.calls)
	}
}

// ---- zfsDatasetForPath ---------------------------------------------------

func TestZfsDatasetForPathExactMount(t *testing.T) {
	prev := zfsListAllFn
	zfsListAllFn = func() (string, error) {
		return "pool/states\t/data/states\npool\t/data\n", nil
	}
	t.Cleanup(func() { zfsListAllFn = prev })

	ds, err := zfsDatasetForPath("/data/states")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds != "pool/states" {
		t.Fatalf("expected pool/states, got %s", ds)
	}
}

func TestZfsDatasetForPathRelative(t *testing.T) {
	prev := zfsListAllFn
	zfsListAllFn = func() (string, error) {
		return "pool\t/data\n", nil
	}
	t.Cleanup(func() { zfsListAllFn = prev })

	ds, err := zfsDatasetForPath("/data/states/base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds != "pool/states/base" {
		t.Fatalf("expected pool/states/base, got %s", ds)
	}
}

func TestZfsDatasetForPathPrefersLongestMount(t *testing.T) {
	prev := zfsListAllFn
	zfsListAllFn = func() (string, error) {
		return "pool\t/data\npool/states\t/data/states\n", nil
	}
	t.Cleanup(func() { zfsListAllFn = prev })

	ds, err := zfsDatasetForPath("/data/states/base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should prefer /data/states over /data as the base mount
	if ds != "pool/states/base" {
		t.Fatalf("expected pool/states/base, got %s", ds)
	}
}

func TestZfsDatasetForPathNotFound(t *testing.T) {
	prev := zfsListAllFn
	zfsListAllFn = func() (string, error) {
		return "pool\t/other\n", nil
	}
	t.Cleanup(func() { zfsListAllFn = prev })

	if _, err := zfsDatasetForPath("/data/states/base"); err == nil {
		t.Fatalf("expected error for unmatched path")
	}
}

func TestZfsDatasetForPathListError(t *testing.T) {
	prev := zfsListAllFn
	zfsListAllFn = func() (string, error) { return "", errors.New("zfs not found") }
	t.Cleanup(func() { zfsListAllFn = prev })

	if _, err := zfsDatasetForPath("/data"); err == nil {
		t.Fatalf("expected error on list failure")
	}
}

// ---- zfsSupported --------------------------------------------------------

func TestZfsSupportedUsesStatfs(t *testing.T) {
	prevStatfs := statfsZfsFn
	prevLookPath := execLookPathZfs
	t.Cleanup(func() { statfsZfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathZfs = prevLookPath })

	var gotPath string
	statfsZfsFn = func(path string, stat *syscall.Statfs_t) error {
		gotPath = path
		stat.Type = zfsSuperMagic
		return nil
	}
	execLookPathZfs = func(string) (string, error) { return "/usr/sbin/zfs", nil }

	if !zfsSupported("/data") {
		t.Fatalf("expected zfs supported")
	}
	if gotPath != "/data" {
		t.Fatalf("expected statfs path /data, got %s", gotPath)
	}
}

func TestZfsSupportedRejectsMissingPath(t *testing.T) {
	if zfsSupported("") {
		t.Fatalf("expected unsupported for empty path")
	}
}

func TestZfsSupportedMissingBinary(t *testing.T) {
	prevLookPath := execLookPathZfs
	prevStatfs := statfsZfsFn
	execLookPathZfs = func(string) (string, error) { return "", errors.New("missing") }
	statfsZfsFn = func(string, *syscall.Statfs_t) error {
		t.Fatalf("statfs should not be called when zfs binary is missing")
		return nil
	}
	t.Cleanup(func() { execLookPathZfs = prevLookPath })
	t.Cleanup(func() { statfsZfsFn = prevStatfs })

	if zfsSupported("/data") {
		t.Fatalf("expected unsupported without zfs binary")
	}
}

func TestZfsSupportedStatfsError(t *testing.T) {
	prevStatfs := statfsZfsFn
	prevLookPath := execLookPathZfs
	statfsZfsFn = func(string, *syscall.Statfs_t) error { return errors.New("boom") }
	execLookPathZfs = func(string) (string, error) { return "/usr/sbin/zfs", nil }
	t.Cleanup(func() { statfsZfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathZfs = prevLookPath })

	if zfsSupported("/data") {
		t.Fatalf("expected unsupported on statfs error")
	}
}

func TestZfsSupportedNonZfs(t *testing.T) {
	prevStatfs := statfsZfsFn
	prevLookPath := execLookPathZfs
	statfsZfsFn = func(path string, stat *syscall.Statfs_t) error {
		stat.Type = 0
		return nil
	}
	execLookPathZfs = func(string) (string, error) { return "/usr/sbin/zfs", nil }
	t.Cleanup(func() { statfsZfsFn = prevStatfs })
	t.Cleanup(func() { execLookPathZfs = prevLookPath })

	if zfsSupported("/data") {
		t.Fatalf("expected unsupported on non-zfs fs")
	}
}

// ---- helpers -------------------------------------------------------------

type zfsFakeFileInfo struct{}

func (zfsFakeFileInfo) Name() string       { return "fake" }
func (zfsFakeFileInfo) Size() int64        { return 0 }
func (zfsFakeFileInfo) Mode() os.FileMode  { return 0 }
func (zfsFakeFileInfo) ModTime() time.Time { return time.Time{} }
func (zfsFakeFileInfo) IsDir() bool        { return true }
func (zfsFakeFileInfo) Sys() any           { return nil }
