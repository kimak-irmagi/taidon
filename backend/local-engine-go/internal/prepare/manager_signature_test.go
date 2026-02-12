package prepare

import (
	"context"
	"errors"
	"testing"

	"sqlrs/engine/internal/prepare/queue"
)

func TestComputeJobSignatureFromPlanMissingImage(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}
	if _, errResp := mgr.computeJobSignatureFromPlan(prepared, nil); errResp == nil {
		t.Fatalf("expected missing image error")
	}
}

func TestComputeJobSignatureFromPlanSuccess(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{})
	prepared := preparedRequest{request: Request{PrepareKind: "lb", ImageID: "img-1", PlanOnly: true}}
	tasks := []PlanTask{
		{
			TaskID:          "task-1",
			Type:            "psql",
			TaskHash:        "hash-1",
			ChangesetID:     "c1",
			ChangesetAuthor: "a1",
			ChangesetPath:   "/tmp/change.sql",
			Input:           &TaskInput{Kind: "state", ID: "state-1"},
		},
	}
	sig, errResp := mgr.computeJobSignatureFromPlan(prepared, tasks)
	if errResp != nil || sig == "" {
		t.Fatalf("expected signature, got sig=%q err=%v", sig, errResp)
	}
}

func TestUpdateJobSignatureFromPlanUpdateError(t *testing.T) {
	queueStore := &faultQueueStore{
		Store: newQueueStore(t),
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	prepared := preparedRequest{request: Request{PrepareKind: "lb", ImageID: "img-1"}}
	errResp := mgr.updateJobSignatureFromPlan(context.Background(), "job-1", prepared, nil)
	if errResp == nil || errResp.Message == "" {
		t.Fatalf("expected update error")
	}
}

func TestUsesContainerLiquibaseRunner(t *testing.T) {
	if !usesContainerLiquibaseRunner(containerLiquibaseRunner{}) {
		t.Fatalf("expected container runner true")
	}
	if usesContainerLiquibaseRunner(hostLiquibaseRunner{}) {
		t.Fatalf("expected host runner false")
	}
}

func TestBuildPlanUnsupportedKind(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{})
	prepared := preparedRequest{request: Request{PrepareKind: "unknown"}}
	if _, _, errResp := mgr.buildPlan(context.Background(), "job-1", prepared); errResp == nil {
		t.Fatalf("expected unsupported prepare kind")
	}
}
