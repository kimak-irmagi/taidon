package statefs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/snapshot"
)

type fakeBackend struct {
	kind           string
	caps           snapshot.Capabilities
	ensureCalls    []string
	destroyCalls   []string
	isSubvolume    bool
	isSubvolumeErr error
}

func (f *fakeBackend) Kind() string {
	return f.kind
}

func (f *fakeBackend) Capabilities() snapshot.Capabilities {
	return f.caps
}

func (f *fakeBackend) Clone(ctx context.Context, srcDir string, destDir string) (snapshot.CloneResult, error) {
	return snapshot.CloneResult{}, nil
}

func (f *fakeBackend) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return nil
}

func (f *fakeBackend) Destroy(ctx context.Context, dir string) error {
	f.destroyCalls = append(f.destroyCalls, dir)
	return nil
}

func (f *fakeBackend) EnsureSubvolume(ctx context.Context, path string) error {
	f.ensureCalls = append(f.ensureCalls, path)
	return nil
}

func (f *fakeBackend) IsSubvolume(ctx context.Context, path string) (bool, error) {
	return f.isSubvolume, f.isSubvolumeErr
}

type fakeBackendNoEnsure struct {
	kind string
}

func (f *fakeBackendNoEnsure) Kind() string {
	return f.kind
}

func (f *fakeBackendNoEnsure) Capabilities() snapshot.Capabilities {
	return snapshot.Capabilities{}
}

func (f *fakeBackendNoEnsure) Clone(ctx context.Context, srcDir string, destDir string) (snapshot.CloneResult, error) {
	return snapshot.CloneResult{}, nil
}

func (f *fakeBackendNoEnsure) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	return nil
}

func (f *fakeBackendNoEnsure) Destroy(ctx context.Context, dir string) error {
	return nil
}

func TestParseImageIDVariants(t *testing.T) {
	engine, tag := parseImageID("")
	if engine != "unknown" || tag != "latest" {
		t.Fatalf("unexpected defaults: %s %s", engine, tag)
	}
	engine, tag = parseImageID("postgres")
	if engine != "postgres" || tag != "latest" {
		t.Fatalf("unexpected parse: %s %s", engine, tag)
	}
	engine, tag = parseImageID("repo/postgres:15")
	if engine != "postgres" || tag != "15" {
		t.Fatalf("unexpected parse: %s %s", engine, tag)
	}
	engine, tag = parseImageID("repo/postgres@sha256:abc")
	if engine != "postgres" || tag != "sha256_abc" {
		t.Fatalf("unexpected parse: %s %s", engine, tag)
	}
	engine, tag = parseImageID("repo/postgres:16@sha256:abc")
	if engine != "postgres_16" || tag != "sha256_abc" {
		t.Fatalf("unexpected parse: %s %s", engine, tag)
	}
	engine, tag = parseImageID("repo/po$stgres:1.0")
	if engine != "po_stgres" || tag != "1.0" {
		t.Fatalf("unexpected parse: %s %s", engine, tag)
	}
}

func TestDirBuildersErrors(t *testing.T) {
	if _, err := baseDir("", "image"); err == nil {
		t.Fatalf("expected baseDir error for empty root")
	}
	if _, err := statesDir("", "image"); err == nil {
		t.Fatalf("expected statesDir error for empty root")
	}
	if _, err := stateDir("", "image", "state"); err == nil {
		t.Fatalf("expected stateDir error for empty root")
	}
	if _, err := stateDir(t.TempDir(), "image", ""); err == nil {
		t.Fatalf("expected stateDir error for empty state id")
	}
	if _, err := jobRuntimeDir("", "job"); err == nil {
		t.Fatalf("expected jobRuntimeDir error for empty root")
	}
	if _, err := jobRuntimeDir(t.TempDir(), ""); err == nil {
		t.Fatalf("expected jobRuntimeDir error for empty job id")
	}
}

func TestEnsureDirsUsesSubvolumeEnsurer(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	state := filepath.Join(root, "state")
	backend := &fakeBackend{kind: "btrfs"}
	mgr := &Manager{backend: backend}
	if err := mgr.EnsureBaseDir(context.Background(), base); err != nil {
		t.Fatalf("EnsureBaseDir: %v", err)
	}
	if err := mgr.EnsureStateDir(context.Background(), state); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}
	if len(backend.ensureCalls) != 2 {
		t.Fatalf("expected ensure calls, got %+v", backend.ensureCalls)
	}
}

func TestEnsureDirsFallbackToMkdirAll(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	state := filepath.Join(root, "state")
	mgr := &Manager{backend: &fakeBackendNoEnsure{}}
	if err := mgr.EnsureBaseDir(context.Background(), base); err != nil {
		t.Fatalf("EnsureBaseDir: %v", err)
	}
	if err := mgr.EnsureStateDir(context.Background(), state); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("expected base dir created: %v", err)
	}
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("expected state dir created: %v", err)
	}
}

func TestRemovePathBtrfsSubvolume(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	backend := &fakeBackend{kind: "btrfs", isSubvolume: true}
	mgr := &Manager{backend: backend}
	if err := mgr.RemovePath(context.Background(), dir); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}
	if len(backend.destroyCalls) != 1 || backend.destroyCalls[0] != dir {
		t.Fatalf("expected destroy call, got %+v", backend.destroyCalls)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected path removed")
	}
}

func TestRemovePathFallbackToDestroy(t *testing.T) {
	backend := &fakeBackend{kind: "copy"}
	mgr := &Manager{backend: backend}
	calls := 0
	prevRemove := removeAll
	removeAll = func(path string) error {
		calls++
		if calls == 1 {
			return os.ErrInvalid
		}
		return nil
	}
	t.Cleanup(func() { removeAll = prevRemove })

	if err := mgr.RemovePath(context.Background(), "dir"); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}
	if len(backend.destroyCalls) != 1 {
		t.Fatalf("expected destroy fallback, got %+v", backend.destroyCalls)
	}
	if calls < 2 {
		t.Fatalf("expected retry removeAll, got %d calls", calls)
	}
}

func TestRemovePathIgnoresEmpty(t *testing.T) {
	backend := &fakeBackend{kind: "btrfs", isSubvolume: true}
	mgr := &Manager{backend: backend}
	if err := mgr.RemovePath(context.Background(), "   "); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}
	if len(backend.destroyCalls) != 0 {
		t.Fatalf("expected no destroy calls, got %+v", backend.destroyCalls)
	}
}

func TestManagerErrorsWithoutBackend(t *testing.T) {
	mgr := &Manager{}
	if err := mgr.Validate(t.TempDir()); err == nil {
		t.Fatalf("expected validate error")
	}
	if _, err := mgr.Clone(context.Background(), "src", "dest"); err == nil {
		t.Fatalf("expected clone error")
	}
	if err := mgr.Snapshot(context.Background(), "src", "dest"); err == nil {
		t.Fatalf("expected snapshot error")
	}
	if err := mgr.EnsureBaseDir(context.Background(), t.TempDir()); err == nil {
		t.Fatalf("expected ensure base error")
	}
}
