package prepare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sqlrs/engine/internal/store"
)

func TestExecuteStateTaskRejectsCancelledContext(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
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
			TaskID: "execute-0",
			Input:  &TaskInput{Kind: "image", ID: "image-1"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, errResp := mgr.executeStateTask(ctx, "job-1", prepared, task)
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestExecuteStateTaskRequiresInput(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, taskState{
		PlanTask: PlanTask{TaskID: "execute-0"},
	})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "task input is required") {
		t.Fatalf("expected missing input error, got %+v", errResp)
	}
}

func TestExecuteStateTaskRejectsUnreadablePsqlInputs(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	missing := filepath.Join(t.TempDir(), "missing.sql")
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "psql",
			ImageID:     "image-1",
		},
		psqlInputs: []psqlInput{
			{kind: "file", value: missing},
		},
	}
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "image", ID: "image-1"},
		},
	}

	_, errResp := mgr.executeStateTask(context.Background(), "job-1", prepared, task)
	if errResp == nil || errResp.Code != "invalid_argument" || !strings.Contains(errResp.Message, "cannot compute psql content hash") {
		t.Fatalf("expected psql content hash error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsCancelledContext(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, errResp := mgr.startRuntime(ctx, "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestStartRuntimeRequiresInput(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, nil)
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "task input is required") {
		t.Fatalf("expected missing input error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsMissingResolvedImage(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{PrepareKind: "psql"},
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected missing resolved image error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsStateInputWithoutID(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: " "})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "input state id is required") {
		t.Fatalf("expected missing state id error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsUnsupportedInputKind(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "snapshot", ID: "state-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "unsupported task input") {
		t.Fatalf("expected unsupported input error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsInputStateLoadError(t *testing.T) {
	mgr := newManager(t, &fakeStore{getStateErr: errors.New("store failed")})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: "state-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "cannot load input state") {
		t.Fatalf("expected input state load error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsMissingInputState(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: "state-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "input state not found") {
		t.Fatalf("expected missing input state error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsStateSnapshotWithoutPGVersion(t *testing.T) {
	st := &fakeStore{
		statesByID: map[string]store.StateEntry{
			"state-1": {
				StateID: "state-1",
				ImageID: "image-1",
			},
		},
	}
	mgr := newManagerWithStateFS(t, st, &fakeStateFS{})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: "state-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "state snapshot missing PG_VERSION") {
		t.Fatalf("expected missing PG_VERSION error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsCloneError(t *testing.T) {
	st := &fakeStore{
		statesByID: map[string]store.StateEntry{
			"state-1": {
				StateID: "state-1",
				ImageID: "image-1",
			},
		},
	}
	mgr := newManagerWithStateFS(t, st, &fakeStateFS{cloneErr: errors.New("clone failed")})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "state", ID: "state-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "cannot clone state") {
		t.Fatalf("expected clone error, got %+v", errResp)
	}
}

func TestStartRuntimeRejectsRuntimeStartError(t *testing.T) {
	rt := &fakeRuntime{startErr: errors.New("docker start failed")}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		runtime: rt,
		statefs: &fakeStateFS{},
	})
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	_, errResp := mgr.startRuntime(context.Background(), "job-1", prepared, &TaskInput{Kind: "image", ID: "image-1"})
	if errResp == nil || errResp.Code != "internal_error" || !strings.Contains(errResp.Message, "cannot start runtime") {
		t.Fatalf("expected runtime start error, got %+v", errResp)
	}
}
