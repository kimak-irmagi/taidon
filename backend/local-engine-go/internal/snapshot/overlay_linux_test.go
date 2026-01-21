//go:build linux

package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []runCall
	err   error
}

type runCall struct {
	name string
	args []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string) error {
	f.calls = append(f.calls, runCall{name: name, args: append([]string{}, args...)})
	return f.err
}

func TestOverlayManagerCloneRequiresDirs(t *testing.T) {
	mgr := overlayManager{runner: &fakeRunner{}}
	if _, err := mgr.Clone(context.Background(), "", "dest"); err == nil {
		t.Fatalf("expected error for missing src")
	}
	if _, err := mgr.Clone(context.Background(), "src", ""); err == nil {
		t.Fatalf("expected error for missing dest")
	}
}

func TestOverlayManagerCloneAndCleanup(t *testing.T) {
	runner := &fakeRunner{}
	mgr := overlayManager{runner: runner}
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "overlay")

	res, err := mgr.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if res.MountDir != filepath.Join(dest, "merged") {
		t.Fatalf("unexpected mount dir: %s", res.MountDir)
	}
	if len(runner.calls) != 1 || runner.calls[0].name != "mount" {
		t.Fatalf("expected mount call, got %+v", runner.calls)
	}
	if err := res.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if len(runner.calls) != 2 || runner.calls[1].name != "umount" {
		t.Fatalf("expected umount call, got %+v", runner.calls)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected dest to be removed")
	}
}

func TestOverlayManagerDestroy(t *testing.T) {
	runner := &fakeRunner{}
	mgr := overlayManager{runner: runner}
	dir := filepath.Join(t.TempDir(), "overlay")
	if err := os.MkdirAll(filepath.Join(dir, "merged"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := mgr.Destroy(context.Background(), dir); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0].name != "umount" {
		t.Fatalf("expected umount call, got %+v", runner.calls)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected dir to be removed")
	}
}

func TestExecRunnerIncludesOutputOnError(t *testing.T) {
	runner := execRunner{}
	err := runner.Run(context.Background(), "sh", []string{"-c", "echo err 1>&2; exit 1"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "err") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}

func TestNewOverlayManagerKind(t *testing.T) {
	manager := newOverlayManager()
	if manager.Kind() != "overlayfs" {
		t.Fatalf("expected overlayfs manager, got %s", manager.Kind())
	}
}

func TestOverlaySupportedMissingMount(t *testing.T) {
	t.Setenv("PATH", "")
	if overlaySupported() {
		t.Fatalf("expected overlay unsupported without mount binary")
	}
}

func TestOverlayManagerSnapshotCopies(t *testing.T) {
	manager := overlayManager{runner: &fakeRunner{}}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.Snapshot(context.Background(), src, dest); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "file.txt")); err != nil {
		t.Fatalf("expected snapshot file: %v", err)
	}
}
