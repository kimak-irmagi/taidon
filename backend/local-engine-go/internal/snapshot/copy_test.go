package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
