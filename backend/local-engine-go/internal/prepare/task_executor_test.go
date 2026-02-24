package prepare

import (
	"context"
	"testing"

	"sqlrs/engine/internal/prepare/queue"
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
