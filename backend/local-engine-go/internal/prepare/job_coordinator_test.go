package prepare

import (
	"context"
	"testing"
)

func TestJobCoordinatorRunJobPlanOnly(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	mgr.coordinator.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job status")
	}
	if status.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
	tasks := mgr.ListTasks("job-1")
	if len(tasks) == 0 {
		t.Fatalf("expected tasks to be created")
	}
	for _, task := range tasks {
		if task.Status != StatusSucceeded {
			t.Fatalf("expected task succeeded, got %q for %s", task.Status, task.TaskID)
		}
	}
}

func TestJobCoordinatorRunJobExecuteFlow(t *testing.T) {
	queueStore := newQueueStore(t)
	store := &fakeStore{}
	mgr := newManagerWithQueue(t, store, queueStore)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}

	mgr.coordinator.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job status")
	}
	if status.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", status.Status)
	}
	if status.Result == nil || status.Result.StateID == "" || status.Result.InstanceID == "" {
		t.Fatalf("expected non-empty result, got %+v", status.Result)
	}
	if len(store.states) == 0 {
		t.Fatalf("expected state to be persisted")
	}
	if len(store.instances) == 0 {
		t.Fatalf("expected instance to be persisted")
	}
	taskRecords, err := queueStore.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(taskRecords) == 0 {
		t.Fatalf("expected persisted tasks")
	}
}
