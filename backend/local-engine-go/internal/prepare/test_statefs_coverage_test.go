package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/statefs"
)

func TestFakeStateFSNilReceiver(t *testing.T) {
	var fs *fakeStateFS
	if fs.Kind() != "" {
		t.Fatalf("expected empty kind")
	}
	if caps := fs.Capabilities(); caps != (statefs.Capabilities{}) {
		t.Fatalf("expected empty caps, got %+v", caps)
	}
	if err := fs.Validate("root"); err != nil {
		t.Fatalf("expected nil validate err, got %v", err)
	}
}

func TestFakeStateFSCloneAndRemove(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "dest")
	fs := &fakeStateFS{mountDir: filepath.Join(dest, "mount")}
	result, err := fs.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if result.MountDir != fs.mountDir {
		t.Fatalf("expected mount dir %q, got %q", fs.mountDir, result.MountDir)
	}
	if _, err := os.Stat(filepath.Join(dest, "PG_VERSION")); err != nil {
		t.Fatalf("expected PG_VERSION copied: %v", err)
	}

	if err := fs.RemovePath(context.Background(), " "); err != nil {
		t.Fatalf("RemovePath blank: %v", err)
	}
	if err := fs.RemovePath(context.Background(), dest); err != nil {
		t.Fatalf("RemovePath: %v", err)
	}
}

func TestFakeStateFSErrors(t *testing.T) {
	fs := &fakeStateFS{cloneErr: os.ErrNotExist, snapshotErr: os.ErrPermission, removeErr: os.ErrPermission}
	if _, err := fs.Clone(context.Background(), "src", "dest"); err == nil {
		t.Fatalf("expected clone error")
	}
	if err := fs.Snapshot(context.Background(), "src", "dest"); err == nil {
		t.Fatalf("expected snapshot error")
	}
	if err := fs.RemovePath(context.Background(), "dest"); err == nil {
		t.Fatalf("expected remove error")
	}
}
