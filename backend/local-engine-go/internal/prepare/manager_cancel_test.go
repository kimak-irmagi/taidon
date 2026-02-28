package prepare

import (
	"context"
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
