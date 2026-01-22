package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	engineRuntime "sqlrs/engine/internal/runtime"
)

func TestFakeRuntimeExec(t *testing.T) {
	rt := &fakeRuntime{}
	if _, err := rt.Exec(context.Background(), "container-1", engineRuntime.ExecRequest{}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestFakeRuntimeWaitForReady(t *testing.T) {
	rt := &fakeRuntime{}
	if err := rt.WaitForReady(context.Background(), "container-1", time.Second); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}
}

func TestFakeRuntimeInitBase(t *testing.T) {
	rt := &fakeRuntime{}
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := rt.InitBase(context.Background(), "image-1", dataDir); err != nil {
		t.Fatalf("InitBase: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "PG_VERSION")); err != nil {
		t.Fatalf("expected PG_VERSION: %v", err)
	}
}

func TestFakeSnapshotCloneDestroy(t *testing.T) {
	snap := &fakeSnapshot{}
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "clone")
	clone, err := snap.Clone(context.Background(), src, dest)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := clone.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected clone dir removed")
	}
	snapshotDir := filepath.Join(t.TempDir(), "snapshot")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	if err := snap.Destroy(context.Background(), snapshotDir); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := os.Stat(snapshotDir); !os.IsNotExist(err) {
		t.Fatalf("expected snapshot dir removed")
	}
}
