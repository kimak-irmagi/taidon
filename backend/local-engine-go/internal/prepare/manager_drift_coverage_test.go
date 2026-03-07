package prepare

import (
	"context"
	"testing"

	"github.com/sqlrs/engine-local/internal/prepare/queue"
)

func TestHasPsqlExecuteShapeDriftCoverage(t *testing.T) {
	if hasPsqlExecuteShapeDrift(preparedRequest{request: Request{PrepareKind: "lb"}}, nil) {
		t.Fatalf("expected no shape drift for non-psql prepare kind")
	}

	if hasPsqlExecuteShapeDrift(preparedRequest{request: Request{PrepareKind: "psql"}}, nil) {
		t.Fatalf("expected no shape drift when psql has implicit single execute task")
	}

	prepared := preparedRequest{
		request:   Request{PrepareKind: "psql"},
		psqlSteps: []psqlStep{{}, {}},
	}

	if !hasPsqlExecuteShapeDrift(prepared, []queue.TaskRecord{{TaskID: "execute-0", Type: "state_execute"}}) {
		t.Fatalf("expected drift when actual execute task count is less than expected")
	}

	if hasPsqlExecuteShapeDrift(prepared, []queue.TaskRecord{
		{TaskID: "execute-0", Type: "state_execute"},
		{TaskID: "execute-1", Type: "state_execute"},
		{TaskID: "execute-2", Type: "state_execute"},
	}) {
		t.Fatalf("expected no drift when execute task count differs but is not a legacy-short shape")
	}

	if !hasPsqlExecuteShapeDrift(prepared, []queue.TaskRecord{
		{TaskID: "execute-0", Type: "state_execute"},
		{TaskID: "execute-2", Type: "state_execute"},
	}) {
		t.Fatalf("expected drift when expected execute task id is missing")
	}

	if hasPsqlExecuteShapeDrift(prepared, []queue.TaskRecord{
		{TaskID: "execute-0", Type: "state_execute"},
		{TaskID: "execute-1", Type: "state_execute"},
	}) {
		t.Fatalf("expected no drift when execute shape matches expected")
	}
}

func TestHasPlanSignatureDriftCoverage(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	drift, errResp := mgr.hasPlanSignatureDrift(preparedRequest{}, queue.JobRecord{}, nil)
	if errResp != nil {
		t.Fatalf("hasPlanSignatureDrift empty signature: %+v", errResp)
	}
	if drift {
		t.Fatalf("expected no drift when stored signature is empty")
	}

	drift, errResp = mgr.hasPlanSignatureDrift(
		preparedRequest{request: Request{PrepareKind: "lb"}},
		queue.JobRecord{Signature: strPtr("sig")},
		nil,
	)
	if errResp == nil {
		t.Fatalf("expected signature computation error for invalid liquibase prepared request")
	}
	if drift {
		t.Fatalf("expected drift=false when signature computation fails")
	}
}

func TestReplanTasksOnDriftReturnsEarlyForEmptyTaskRecords(t *testing.T) {
	var coordinator jobCoordinator
	replanned, tasks, stateID, errResp := coordinator.replanTasksOnDrift(context.Background(), "job-1", preparedRequest{}, nil)
	if errResp != nil {
		t.Fatalf("expected no error response, got %+v", errResp)
	}
	if replanned || tasks != nil || stateID != "" {
		t.Fatalf("unexpected early-return values: replanned=%v tasks=%v stateID=%q", replanned, tasks, stateID)
	}
}
