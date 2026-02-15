package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/store"
)

func TestSnapshotOrchestratorInvalidatesDirtyCachedState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state-store")
	fstore := &fakeStore{
		statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1@sha256:resolved"},
		},
	}
	mgr := newManagerWithDeps(t, fstore, newQueueStore(t), &testDeps{stateRoot: stateRoot})

	stateDir, err := mgr.statefs.StateDir(stateRoot, "image-1@sha256:resolved", "state-1")
	if err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "postmaster.pid"), []byte("1"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prepared := preparedRequest{
		request:         Request{PrepareKind: "psql", ImageID: "image-1"},
		resolvedImageID: "image-1@sha256:resolved",
	}
	dirty, errResp := mgr.snapshot.invalidateDirtyCachedState(context.Background(), "job-1", prepared, "state-1")
	if errResp != nil {
		t.Fatalf("invalidateDirtyCachedState: %+v", errResp)
	}
	if !dirty {
		t.Fatalf("expected dirty state invalidation")
	}
	if len(fstore.deletedStates) != 1 || fstore.deletedStates[0] != "state-1" {
		t.Fatalf("expected state delete call, got %+v", fstore.deletedStates)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("expected state dir removal, stat err=%v", err)
	}
}

func TestSnapshotOrchestratorEnsureBaseStateInitGuard(t *testing.T) {
	rt := &fakeRuntime{}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{runtime: rt})
	baseDir := filepath.Join(t.TempDir(), "base")

	if err := mgr.snapshot.ensureBaseState(context.Background(), "image-1", baseDir); err != nil {
		t.Fatalf("ensureBaseState first call: %v", err)
	}
	if err := mgr.snapshot.ensureBaseState(context.Background(), "image-1", baseDir); err != nil {
		t.Fatalf("ensureBaseState second call: %v", err)
	}
	if len(rt.initCalls) != 1 {
		t.Fatalf("expected single init call due init marker guard, got %d", len(rt.initCalls))
	}
}
