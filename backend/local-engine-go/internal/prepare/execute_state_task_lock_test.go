package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/snapshot"
)

type blockingSnapshot struct {
	fakeSnapshot
	started  chan struct{}
	proceed  chan struct{}
	calls    int
}

func (b *blockingSnapshot) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	b.calls++
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-b.proceed
	return b.fakeSnapshot.Snapshot(ctx, srcDir, destDir)
}

func TestExecuteStateTaskWaitsForStateBuild(t *testing.T) {
	store := &fakeStore{}
	snap := &blockingSnapshot{
		started: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	mgr := newManagerWithSnapshot(t, store, snap)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	firstDone := make(chan *ErrorResponse, 1)
	go func() {
		firstDone <- mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	}()

	select {
	case <-snap.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("snapshot did not start")
	}

	secondDone := make(chan *ErrorResponse, 1)
	go func() {
		secondDone <- mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	}()

	select {
	case <-secondDone:
		t.Fatalf("expected second task to wait")
	case <-time.After(100 * time.Millisecond):
	}
	if snap.calls != 1 {
		t.Fatalf("expected single snapshot call, got %d", snap.calls)
	}

	close(snap.proceed)

	if errResp := <-firstDone; errResp != nil {
		t.Fatalf("first executeStateTask: %+v", errResp)
	}
	if errResp := <-secondDone; errResp != nil {
		t.Fatalf("second executeStateTask: %+v", errResp)
	}
	if snap.calls != 1 {
		t.Fatalf("expected single snapshot call, got %d", snap.calls)
	}
	if len(store.states) != 1 {
		t.Fatalf("expected single state, got %+v", store.states)
	}

	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, task.OutputStateID)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if !stateBuildMarkerExists(paths.stateDir) {
		t.Fatalf("expected build marker")
	}
}

func TestExecuteStateTaskCreatesBuildMarker(t *testing.T) {
	store := &fakeStore{}
	mgr := newManagerWithSnapshot(t, store, &fakeSnapshot{})

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	if errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, task.OutputStateID)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	markerPath := filepath.Join(paths.stateDir, stateBuildMarkerName)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected marker, got %v", err)
	}
}

func newManagerWithSnapshot(t *testing.T, store *fakeStore, snap snapshot.Manager) *Manager {
	t.Helper()
	mgr, err := NewManager(Options{
		Store:          store,
		Queue:          newQueueStore(t),
		Runtime:        &fakeRuntime{},
		Snapshot:       snap,
		DBMS:           &fakeDBMS{},
		StateStoreRoot: filepath.Join(t.TempDir(), "state-store"),
		Version:        "v1",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}
