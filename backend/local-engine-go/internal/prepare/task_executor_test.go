package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sqlrs/engine/internal/prepare/queue"
	"sqlrs/engine/internal/store"
)

func TestTaskExecutorEnsureRuntimeReusesRunnerRuntime(t *testing.T) {
	rt := &fakeRuntime{}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{runtime: rt})
	prepared := preparedRequest{
		request:         Request{PrepareKind: "psql", ImageID: "image-1"},
		resolvedImageID: "image-1@sha256:resolved",
	}
	runner := &jobRunner{}
	input := &TaskInput{Kind: "image", ID: prepared.effectiveImageID()}

	first, errResp := mgr.executor.ensureRuntime(context.Background(), "job-1", prepared, input, runner)
	if errResp != nil {
		t.Fatalf("ensureRuntime first call: %+v", errResp)
	}
	second, errResp := mgr.executor.ensureRuntime(context.Background(), "job-1", prepared, input, runner)
	if errResp != nil {
		t.Fatalf("ensureRuntime second call: %+v", errResp)
	}
	if first == nil || second == nil || first != second {
		t.Fatalf("expected runtime to be reused")
	}
	if len(rt.startCalls) != 1 {
		t.Fatalf("expected one runtime start, got %d", len(rt.startCalls))
	}
}

func TestTaskExecutorExecuteStateTaskUpdatesTaskMetadata(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	prepared.resolvedImageID = "image-1@sha256:resolved"
	task := taskState{
		PlanTask: PlanTask{
			TaskID: "execute-0",
			Type:   "state_execute",
			Input: &TaskInput{
				Kind: "image",
				ID:   prepared.effectiveImageID(),
			},
		},
		Status: StatusQueued,
	}
	records := []queue.TaskRecord{
		{
			JobID:     "job-1",
			TaskID:    task.TaskID,
			Position:  0,
			Type:      task.Type,
			Status:    StatusQueued,
			InputKind: strPtr(task.Input.Kind),
			InputID:   strPtr(task.Input.ID),
		},
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", records); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	runner := mgr.registerRunner("job-1", func() {})
	t.Cleanup(func() {
		mgr.cleanupRuntime(context.Background(), runner)
		close(runner.done)
		mgr.unregisterRunner("job-1")
	})

	if _, errResp := mgr.executor.executeStateTask(context.Background(), "job-1", prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	updated, err := queueStore.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected one task record, got %d", len(updated))
	}
	record := updated[0]
	if record.TaskHash == nil || *record.TaskHash == "" {
		t.Fatalf("expected task hash to be stored")
	}
	if record.OutputStateID == nil || *record.OutputStateID == "" {
		t.Fatalf("expected output state id to be stored")
	}
	if record.Cached == nil {
		t.Fatalf("expected cached decision to be stored")
	}
}

func TestTaskExecutorExecuteStateTaskCachedChainDoesNotStartRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	stateStore := &fakeStore{}
	mgr := newManagerWithDeps(t, stateStore, newQueueStore(t), &testDeps{runtime: runtime})
	workDir := t.TempDir()
	step1 := filepath.Join(workDir, "step1.sql")
	step2 := filepath.Join(workDir, "step2.sql")
	if err := os.WriteFile(step1, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write step1: %v", err)
	}
	if err := os.WriteFile(step2, []byte("select 2;"), 0o600); err != nil {
		t.Fatalf("write step2: %v", err)
	}
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f", step1, "-f", step2},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	input0 := TaskInput{Kind: "image", ID: "image-1"}
	state1 := psqlOutputStateIDForStep(t, mgr, prepared, input0, "execute-0")
	input1 := TaskInput{Kind: "state", ID: state1}
	state2 := psqlOutputStateIDForStep(t, mgr, prepared, input1, "execute-1")

	stateStore.statesByID = map[string]store.StateEntry{
		state1: {StateID: state1, ImageID: "image-1"},
		state2: {StateID: state2, ImageID: "image-1"},
	}
	for _, stateID := range []string{state1, state2} {
		paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", stateID, mgr.statefs)
		if err != nil {
			t.Fatalf("resolveStatePaths(%s): %v", stateID, err)
		}
		if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
			t.Fatalf("mkdir state dir %s: %v", stateID, err)
		}
		if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
			t.Fatalf("write PG_VERSION %s: %v", stateID, err)
		}
	}

	task0 := taskState{PlanTask: PlanTask{
		TaskID:        "execute-0",
		Type:          "state_execute",
		OutputStateID: state1,
		Input:         &TaskInput{Kind: "image", ID: "image-1"},
	}}
	got0, errResp := mgr.executor.executeStateTask(context.Background(), "job-1", prepared, task0)
	if errResp != nil {
		t.Fatalf("executeStateTask task0: %+v", errResp)
	}
	if got0 != state1 {
		t.Fatalf("expected %s, got %s", state1, got0)
	}

	task1 := taskState{PlanTask: PlanTask{
		TaskID:        "execute-1",
		Type:          "state_execute",
		OutputStateID: state2,
		Input:         &TaskInput{Kind: "state", ID: state1},
	}}
	got1, errResp := mgr.executor.executeStateTask(context.Background(), "job-1", prepared, task1)
	if errResp != nil {
		t.Fatalf("executeStateTask task1: %+v", errResp)
	}
	if got1 != state2 {
		t.Fatalf("expected %s, got %s", state2, got1)
	}

	if len(runtime.startCalls) != 0 {
		t.Fatalf("expected no runtime starts for cached chain, got %+v", runtime.startCalls)
	}
	if len(runtime.stopCalls) != 0 {
		t.Fatalf("expected no runtime stops for cached chain, got %+v", runtime.stopCalls)
	}
}
