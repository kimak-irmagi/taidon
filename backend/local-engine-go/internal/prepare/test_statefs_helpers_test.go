package prepare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFakeStateFSEnsureBaseDirErrorBranch(t *testing.T) {
	fs := &fakeStateFS{ensureBaseErr: errors.New("boom")}
	if err := fs.EnsureBaseDir(context.Background(), filepath.Join(t.TempDir(), "base")); err == nil {
		t.Fatalf("expected ensure base dir error")
	}
}

func TestFakeStateFSCloneErrorBranches(t *testing.T) {
	fs := &fakeStateFS{}

	t.Run("mkdir failure", func(t *testing.T) {
		srcDir := t.TempDir()
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatalf("write blocker: %v", err)
		}
		_, err := fs.Clone(context.Background(), srcDir, filepath.Join(blocker, "runtime"))
		if err == nil {
			t.Fatalf("expected mkdir failure")
		}
	})

	t.Run("write failure when PG_VERSION target is directory", func(t *testing.T) {
		srcDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(srcDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
			t.Fatalf("write source PG_VERSION: %v", err)
		}
		destDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(destDir, "PG_VERSION"), 0o700); err != nil {
			t.Fatalf("mkdir target PG_VERSION dir: %v", err)
		}
		_, err := fs.Clone(context.Background(), srcDir, destDir)
		if err == nil {
			t.Fatalf("expected write failure for PG_VERSION directory")
		}
	})

	t.Run("read error that is not not-exist", func(t *testing.T) {
		_, err := fs.Clone(context.Background(), "bad\x00src", filepath.Join(t.TempDir(), "runtime"))
		if err == nil {
			t.Fatalf("expected read error")
		}
	})
}
