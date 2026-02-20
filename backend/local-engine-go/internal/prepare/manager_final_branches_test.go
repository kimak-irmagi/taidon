package prepare

import (
	"context"
	"errors"
	"testing"
	"time"

	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare/queue"
)

func TestDeleteReturnsFalseWhenDeleteJobFails(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{PrepareKind: "psql", ImageID: "image-1", PsqlArgs: []string{"-c", "select 1"}}
	createJobRecord(t, queueStore, "job-1", req, StatusSucceeded)

	faulty := &faultQueueStore{
		Store: queueStore,
		deleteJob: func(context.Context, string) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	if _, ok := mgr.Delete("job-1", deletion.DeleteOptions{}); ok {
		t.Fatalf("expected delete failure")
	}
}

func TestLoadOrPlanTasksReturnsUpdateSignatureError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1@sha256:abc",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if _, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared); errResp == nil {
		t.Fatalf("expected update signature error")
	}
}

func TestLoadOrPlanTasksReturnsBuildPlanError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "unsupported",
			ImageID:     "image-1@sha256:abc",
		},
	}
	if _, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared); errResp == nil {
		t.Fatalf("expected build plan error")
	}
}

func TestTrimCompletedJobsBySignatureNoopWhenLimitDisabled(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		config: &fakeConfigStore{value: 0},
	})
	mgr.trimCompletedJobsBySignature(context.Background(), "sig")
}

func TestRecoverRunsQueuedJobAsync(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{PrepareKind: "psql", ImageID: "image-1", PsqlArgs: []string{"-c", "select 1"}}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)

	mgr, err := NewPrepareService(Options{
		Store:          &fakeStore{},
		Queue:          queueStore,
		Runtime:        &fakeRuntime{},
		StateFS:        &fakeStateFS{},
		DBMS:           &fakeDBMS{},
		StateStoreRoot: t.TempDir(),
		Version:        "v1",
		Async:          true,
	})
	if err != nil {
		t.Fatalf("NewPrepareService: %v", err)
	}

	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, ok := mgr.Get("job-1")
		if ok && (status.Status == StatusSucceeded || status.Status == StatusFailed) {
			if status.Status != StatusSucceeded {
				t.Fatalf("expected succeeded status, got %+v", status)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job did not finish in time")
}

func TestWaitForEventReturnsContextErrorOnTimeout(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{PrepareKind: "psql", ImageID: "image-1", PsqlArgs: []string{"-c", "select 1"}}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := mgr.WaitForEvent(ctx, "job-1", 10); err == nil {
		t.Fatalf("expected context timeout error")
	}
}

func TestSucceedReturnsErrorWhenResultEventAppendFails(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
	}
	appendCalls := 0
	faulty.appendEvent = func(ctx context.Context, rec queue.EventRecord) (int64, error) {
		appendCalls++
		if appendCalls == 2 {
			return 0, errors.New("boom")
		}
		return queueStore.AppendEvent(ctx, rec)
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	createJobRecord(t, queueStore, "job-1", Request{PrepareKind: "psql", ImageID: "image-1"}, StatusRunning)

	err := mgr.succeed("job-1", Result{InstanceID: "inst-1", StateID: "state-1", ImageID: "image-1", PrepareKind: "psql"})
	if err == nil {
		t.Fatalf("expected result event append error")
	}
}

func TestSucceedPlanReturnsErrorWhenStatusEventAppendFails(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		appendEvent: func(context.Context, queue.EventRecord) (int64, error) {
			return 0, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	createJobRecord(t, queueStore, "job-1", Request{PrepareKind: "psql", ImageID: "image-1"}, StatusRunning)

	if err := mgr.succeedPlan("job-1"); err == nil {
		t.Fatalf("expected status event append error")
	}
}

func TestFailJobReturnsErrorWhenErrorEventAppendFails(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
	}
	appendCalls := 0
	faulty.appendEvent = func(ctx context.Context, rec queue.EventRecord) (int64, error) {
		appendCalls++
		if appendCalls == 2 {
			return 0, errors.New("boom")
		}
		return queueStore.AppendEvent(ctx, rec)
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	createJobRecord(t, queueStore, "job-1", Request{PrepareKind: "psql", ImageID: "image-1"}, StatusRunning)

	if err := mgr.failJob("job-1", errorResponse("boom", "failed", "")); err == nil {
		t.Fatalf("expected error event append failure")
	}
}
