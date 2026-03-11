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

func TestHasPlanSignatureDriftPsqlMatchAndMismatch(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request:         Request{PrepareKind: "psql"},
		resolvedImageID: "image-1@sha256:resolved",
		psqlInputs:      []psqlInput{{kind: "command", value: "select 1"}},
	}

	signature, errResp := mgr.computeJobSignature(prepared)
	if errResp != nil {
		t.Fatalf("computeJobSignature: %+v", errResp)
	}

	drift, errResp := mgr.hasPlanSignatureDrift(prepared, queue.JobRecord{Signature: strPtr(signature)}, nil)
	if errResp != nil {
		t.Fatalf("hasPlanSignatureDrift match: %+v", errResp)
	}
	if drift {
		t.Fatalf("expected no drift for matching psql signature")
	}

	drift, errResp = mgr.hasPlanSignatureDrift(prepared, queue.JobRecord{Signature: strPtr(signature + "-other")}, nil)
	if errResp != nil {
		t.Fatalf("hasPlanSignatureDrift mismatch: %+v", errResp)
	}
	if !drift {
		t.Fatalf("expected drift for mismatching psql signature")
	}
}

func TestHasPlanSignatureDriftLiquibaseMatchAndMismatch(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request:         Request{PrepareKind: "lb"},
		resolvedImageID: "image-1@sha256:resolved",
	}
	taskRecords := []queue.TaskRecord{
		{
			TaskID:   "execute-0",
			Type:     "state_execute",
			JobID:    "job-1",
			TaskHash: strPtr("hash-1"),
		},
	}
	planTasks := planTasksFromRecords(taskRecords)
	signature, errResp := mgr.computeJobSignatureFromPlan(prepared, planTasks)
	if errResp != nil {
		t.Fatalf("computeJobSignatureFromPlan: %+v", errResp)
	}

	drift, errResp := mgr.hasPlanSignatureDrift(prepared, queue.JobRecord{Signature: strPtr(signature)}, taskRecords)
	if errResp != nil {
		t.Fatalf("hasPlanSignatureDrift lb match: %+v", errResp)
	}
	if drift {
		t.Fatalf("expected no drift for matching lb signature")
	}

	drift, errResp = mgr.hasPlanSignatureDrift(prepared, queue.JobRecord{Signature: strPtr(signature + "-other")}, taskRecords)
	if errResp != nil {
		t.Fatalf("hasPlanSignatureDrift lb mismatch: %+v", errResp)
	}
	if !drift {
		t.Fatalf("expected drift for mismatching lb signature")
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
