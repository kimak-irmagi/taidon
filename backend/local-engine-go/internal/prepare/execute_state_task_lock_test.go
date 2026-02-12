package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/statefs"
)

type blockingSnapshot struct {
	fakeStateFS
	started chan struct{}
	proceed chan struct{}
	calls   int
}

func (b *blockingSnapshot) Snapshot(ctx context.Context, srcDir string, destDir string) error {
	b.calls++
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-b.proceed
	return b.fakeStateFS.Snapshot(ctx, srcDir, destDir)
}

func TestExecuteStateTaskWaitsForStateBuild(t *testing.T) {
	store := &fakeStore{}
	snap := &blockingSnapshot{
		started: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	mgr := newManagerWithStateFS(t, store, snap)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	firstDone := make(chan *ErrorResponse, 1)
	go func() {
		_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
		firstDone <- errResp
	}()

	select {
	case <-snap.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("snapshot did not start")
	}

	secondDone := make(chan *ErrorResponse, 1)
	go func() {
		_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
		secondDone <- errResp
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

	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if !stateBuildMarkerExists(paths.stateDir, snapshotKind(mgr.statefs)) {
		t.Fatalf("expected build marker")
	}
}

func TestExecuteStateTaskCreatesBuildMarker(t *testing.T) {
	store := &fakeStore{}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	if _, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	markerPath := stateBuildMarkerPath(paths.stateDir, snapshotKind(mgr.statefs))
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected marker, got %v", err)
	}
}

func TestExecuteStateTaskRebuildsWhenMarkerStale(t *testing.T) {
	store := &fakeStore{}
	snap := &fakeStateFS{}
	mgr := newManagerWithStateFS(t, store, snap)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stalePath := filepath.Join(paths.stateDir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if err := os.WriteFile(stateBuildMarkerPath(paths.stateDir, snapshotKind(mgr.statefs)), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if _, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if len(snap.snapshotCalls) != 1 {
		t.Fatalf("expected snapshot, got %+v", snap.snapshotCalls)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed")
	}
	if len(store.states) != 1 {
		t.Fatalf("expected state to be stored, got %+v", store.states)
	}
}

func newManagerWithStateFS(t *testing.T, store *fakeStore, fs statefs.StateFS) *Manager {
	t.Helper()
	mgr, err := NewManager(Options{
		Store:          store,
		Queue:          newQueueStore(t),
		Runtime:        &fakeRuntime{},
		StateFS:        fs,
		DBMS:           &fakeDBMS{},
		StateStoreRoot: filepath.Join(t.TempDir(), "state-store"),
		Version:        "v1",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

type btrfsSnapshot struct {
	fakeStateFS
}

func (b *btrfsSnapshot) Kind() string {
	return "btrfs"
}

func TestExecuteStateTaskCleansNonSubvolumeStateDirForBtrfs(t *testing.T) {
	store := &fakeStore{}
	snap := &btrfsSnapshot{}
	mgr := newManagerWithStateFS(t, store, snap)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stalePath := filepath.Join(paths.stateDir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}

	if _, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed")
	}
	if len(store.states) != 1 {
		t.Fatalf("expected state to be stored, got %+v", store.states)
	}
}

func TestExecuteStateTaskDestroysSubvolumeStateDirForBtrfs(t *testing.T) {
	store := &fakeStore{}
	snap := &btrfsSnapshot{}
	mgr := newManagerWithStateFS(t, store, snap)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	outputID := psqlOutputStateID(t, mgr, prepared, TaskInput{Kind: "image", ID: "image-1"})
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: outputID,
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	paths, err := resolveStatePaths(mgr.stateStoreRoot, prepared.request.ImageID, outputID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if len(snap.removeCalls) == 0 {
		t.Fatalf("expected remove to be called")
	}
}
