package prepare

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"sqlrs/engine/internal/prepare/queue"
)

func TestCancelQueuedWithoutRunnerMarksFailed(t *testing.T) {
	mgr := newManagerWithQueue(t, &fakeStore{}, newQueueStore(t))
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := mgr.queue.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-queued",
		Status:      StatusQueued,
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	status, found, accepted, err := mgr.Cancel("job-queued")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Fatalf("expected job to be found")
	}
	if !accepted {
		t.Fatalf("expected cancel request to be accepted")
	}
	if status.Status != StatusFailed {
		t.Fatalf("expected failed status, got %q", status.Status)
	}
	if status.Error == nil || status.Error.Code != "cancelled" {
		t.Fatalf("expected cancelled error payload, got %+v", status.Error)
	}
}

func TestCancelTerminalJobNoop(t *testing.T) {
	mgr := newManagerWithQueue(t, &fakeStore{}, newQueueStore(t))
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	finishedAt := createdAt
	if err := mgr.queue.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-terminal",
		Status:      StatusSucceeded,
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   createdAt,
		FinishedAt:  &finishedAt,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	status, found, accepted, err := mgr.Cancel("job-terminal")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Fatalf("expected job to be found")
	}
	if accepted {
		t.Fatalf("expected terminal job cancel to be a no-op")
	}
	if status.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
}

func TestCancelNotFound(t *testing.T) {
	mgr := newManagerWithQueue(t, &fakeStore{}, newQueueStore(t))

	status, found, accepted, err := mgr.Cancel("missing-job")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if found {
		t.Fatalf("expected missing job")
	}
	if accepted {
		t.Fatalf("expected accepted=false for missing job")
	}
	if status.JobID != "" {
		t.Fatalf("expected empty status for missing job, got %+v", status)
	}
}

func TestCancelRunningWithActiveRunner(t *testing.T) {
	mgr := newManagerWithQueue(t, &fakeStore{}, newQueueStore(t))
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := mgr.queue.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-running",
		Status:      StatusRunning,
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   createdAt,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	cancelCalled := false
	runner := mgr.registerRunner("job-running", func() { cancelCalled = true })
	defer mgr.unregisterRunner("job-running")
	defer close(runner.done)

	status, found, accepted, err := mgr.Cancel("job-running")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Fatalf("expected job to be found")
	}
	if !accepted {
		t.Fatalf("expected cancel request to be accepted")
	}
	if !cancelCalled {
		t.Fatalf("expected active runner cancel to be called")
	}
	if status.Status != StatusRunning {
		t.Fatalf("expected running status snapshot after cancel request, got %q", status.Status)
	}
}

func TestCancelGetJobError(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{}, false, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	_, found, accepted, err := mgr.Cancel("job-1")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected get job error, got %v", err)
	}
	if found {
		t.Fatalf("expected found=false on get error")
	}
	if accepted {
		t.Fatalf("expected accepted=false on get error")
	}
}

func TestCancelTerminalGetReturnsMissing(t *testing.T) {
	var calls int32
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			if atomic.AddInt32(&calls, 1) == 1 {
				createdAt := time.Now().UTC().Format(time.RFC3339Nano)
				return queue.JobRecord{
					JobID:       "job-terminal-missing",
					Status:      StatusSucceeded,
					PrepareKind: "psql",
					ImageID:     "image-1",
					CreatedAt:   createdAt,
				}, true, nil
			}
			return queue.JobRecord{}, false, nil
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	status, found, accepted, err := mgr.Cancel("job-terminal-missing")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if accepted {
		t.Fatalf("expected accepted=false for terminal status")
	}
	if status.JobID != "" {
		t.Fatalf("expected empty status when Get returns missing, got %+v", status)
	}
}

func TestCancelRunningWithRunnerAndMissingStatusAfterCancel(t *testing.T) {
	var calls int32
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			if atomic.AddInt32(&calls, 1) == 1 {
				createdAt := time.Now().UTC().Format(time.RFC3339Nano)
				return queue.JobRecord{
					JobID:       "job-runner-missing",
					Status:      StatusRunning,
					PrepareKind: "psql",
					ImageID:     "image-1",
					CreatedAt:   createdAt,
				}, true, nil
			}
			return queue.JobRecord{}, false, nil
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	runner := mgr.registerRunner("job-runner-missing", func() {})
	defer mgr.unregisterRunner("job-runner-missing")
	defer close(runner.done)

	status, found, accepted, err := mgr.Cancel("job-runner-missing")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if !accepted {
		t.Fatalf("expected accepted=true")
	}
	if status.JobID != "" {
		t.Fatalf("expected empty status when Get returns missing, got %+v", status)
	}
}

func TestCancelQueuedWithoutRunnerFailJobError(t *testing.T) {
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{
				JobID:       "job-fail-error",
				Status:      StatusQueued,
				PrepareKind: "psql",
				ImageID:     "image-1",
				CreatedAt:   createdAt,
			}, true, nil
		},
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("update failed")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	_, found, accepted, err := mgr.Cancel("job-fail-error")
	if err == nil || err.Error() != "update failed" {
		t.Fatalf("expected update error, got %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if accepted {
		t.Fatalf("expected accepted=false on failJob error")
	}
}
