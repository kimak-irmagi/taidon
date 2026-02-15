package prepare

import (
	"context"
	"testing"
)

type coordinatorSpy struct {
	runJobCalled                  bool
	loadOrPlanTasksCalled         bool
	buildPlanCalled               bool
	buildPlanPsqlCalled           bool
	buildPlanLiquibaseCalled      bool
	planLiquibaseChangesetsCalled bool
}

func (s *coordinatorSpy) runJob(prepared preparedRequest, jobID string) {
	s.runJobCalled = true
}

func (s *coordinatorSpy) loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse) {
	s.loadOrPlanTasksCalled = true
	return nil, "state-1", nil
}

func (s *coordinatorSpy) buildPlan(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	s.buildPlanCalled = true
	return nil, "state-2", nil
}

func (s *coordinatorSpy) buildPlanPsql(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	s.buildPlanPsqlCalled = true
	return nil, "state-3", nil
}

func (s *coordinatorSpy) buildPlanLiquibase(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	s.buildPlanLiquibaseCalled = true
	return nil, "state-4", nil
}

func (s *coordinatorSpy) planLiquibaseChangesets(ctx context.Context, jobID string, prepared preparedRequest) ([]LiquibaseChangeset, *ErrorResponse) {
	s.planLiquibaseChangesetsCalled = true
	return nil, nil
}

type executorSpy struct {
	executeStateTaskCalled     bool
	executePrepareStepCalled   bool
	executePsqlStepCalled      bool
	executeLiquibaseStepCalled bool
	runUpdateSQLCalled         bool
	createInstanceCalled       bool
	ensureRuntimeCalled        bool
	startRuntimeCalled         bool
}

func (s *executorSpy) executeStateTask(ctx context.Context, jobID string, prepared preparedRequest, task taskState) (string, *ErrorResponse) {
	s.executeStateTaskCalled = true
	return "state-1", nil
}

func (s *executorSpy) executePrepareStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	s.executePrepareStepCalled = true
	return nil
}

func (s *executorSpy) executePsqlStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) *ErrorResponse {
	s.executePsqlStepCalled = true
	return nil
}

func (s *executorSpy) executeLiquibaseStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	s.executeLiquibaseStepCalled = true
	return nil
}

func (s *executorSpy) runLiquibaseUpdateSQL(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) ([]LiquibaseChangeset, *ErrorResponse) {
	s.runUpdateSQLCalled = true
	return nil, nil
}

func (s *executorSpy) createInstance(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (*Result, *ErrorResponse) {
	s.createInstanceCalled = true
	return &Result{}, nil
}

func (s *executorSpy) ensureRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput, runner *jobRunner) (*jobRuntime, *ErrorResponse) {
	s.ensureRuntimeCalled = true
	return &jobRuntime{}, nil
}

func (s *executorSpy) startRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput) (*jobRuntime, *ErrorResponse) {
	s.startRuntimeCalled = true
	return &jobRuntime{}, nil
}

type snapshotSpy struct {
	ensureBaseStateCalled bool
	invalidateCalled      bool
}

func (s *snapshotSpy) ensureBaseState(ctx context.Context, imageID string, baseDir string) error {
	s.ensureBaseStateCalled = true
	return nil
}

func (s *snapshotSpy) invalidateDirtyCachedState(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (bool, *ErrorResponse) {
	s.invalidateCalled = true
	return false, nil
}

func TestManagerDelegatesToCoordinator(t *testing.T) {
	mgr := &PrepareService{}
	coordinator := &coordinatorSpy{}
	mgr.coordinator = coordinator

	mgr.runJob(preparedRequest{}, "job-1")
	if !coordinator.runJobCalled {
		t.Fatalf("expected runJob delegation")
	}
	if _, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", preparedRequest{}); errResp != nil {
		t.Fatalf("unexpected loadOrPlanTasks error: %+v", errResp)
	}
	if !coordinator.loadOrPlanTasksCalled {
		t.Fatalf("expected loadOrPlanTasks delegation")
	}
	if _, _, errResp := mgr.buildPlan(context.Background(), "job-1", preparedRequest{}); errResp != nil {
		t.Fatalf("unexpected buildPlan error: %+v", errResp)
	}
	if !coordinator.buildPlanCalled {
		t.Fatalf("expected buildPlan delegation")
	}
	if _, _, errResp := mgr.buildPlanPsql(preparedRequest{}); errResp != nil {
		t.Fatalf("unexpected buildPlanPsql error: %+v", errResp)
	}
	if !coordinator.buildPlanPsqlCalled {
		t.Fatalf("expected buildPlanPsql delegation")
	}
	if _, _, errResp := mgr.buildPlanLiquibase(context.Background(), "job-1", preparedRequest{}); errResp != nil {
		t.Fatalf("unexpected buildPlanLiquibase error: %+v", errResp)
	}
	if !coordinator.buildPlanLiquibaseCalled {
		t.Fatalf("expected buildPlanLiquibase delegation")
	}
	if _, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", preparedRequest{}); errResp != nil {
		t.Fatalf("unexpected planLiquibaseChangesets error: %+v", errResp)
	}
	if !coordinator.planLiquibaseChangesetsCalled {
		t.Fatalf("expected planLiquibaseChangesets delegation")
	}
}

func TestManagerDelegatesToExecutorAndSnapshot(t *testing.T) {
	mgr := &PrepareService{}
	executor := &executorSpy{}
	snapshot := &snapshotSpy{}
	mgr.executor = executor
	mgr.snapshot = snapshot

	if _, errResp := mgr.executeStateTask(context.Background(), "job-1", preparedRequest{}, taskState{}); errResp != nil {
		t.Fatalf("unexpected executeStateTask error: %+v", errResp)
	}
	if !executor.executeStateTaskCalled {
		t.Fatalf("expected executeStateTask delegation")
	}
	if errResp := mgr.executePrepareStep(context.Background(), "job-1", preparedRequest{}, &jobRuntime{}, taskState{}); errResp != nil {
		t.Fatalf("unexpected executePrepareStep error: %+v", errResp)
	}
	if !executor.executePrepareStepCalled {
		t.Fatalf("expected executePrepareStep delegation")
	}
	if errResp := mgr.executePsqlStep(context.Background(), "job-1", preparedRequest{}, &jobRuntime{}); errResp != nil {
		t.Fatalf("unexpected executePsqlStep error: %+v", errResp)
	}
	if !executor.executePsqlStepCalled {
		t.Fatalf("expected executePsqlStep delegation")
	}
	if errResp := mgr.executeLiquibaseStep(context.Background(), "job-1", preparedRequest{}, &jobRuntime{}, taskState{}); errResp != nil {
		t.Fatalf("unexpected executeLiquibaseStep error: %+v", errResp)
	}
	if !executor.executeLiquibaseStepCalled {
		t.Fatalf("expected executeLiquibaseStep delegation")
	}
	if _, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", preparedRequest{}, &jobRuntime{}); errResp != nil {
		t.Fatalf("unexpected runLiquibaseUpdateSQL error: %+v", errResp)
	}
	if !executor.runUpdateSQLCalled {
		t.Fatalf("expected runLiquibaseUpdateSQL delegation")
	}
	if _, errResp := mgr.createInstance(context.Background(), "job-1", preparedRequest{}, "state-1"); errResp != nil {
		t.Fatalf("unexpected createInstance error: %+v", errResp)
	}
	if !executor.createInstanceCalled {
		t.Fatalf("expected createInstance delegation")
	}
	if _, errResp := mgr.ensureRuntime(context.Background(), "job-1", preparedRequest{}, &TaskInput{Kind: "image", ID: "image-1"}, &jobRunner{}); errResp != nil {
		t.Fatalf("unexpected ensureRuntime error: %+v", errResp)
	}
	if !executor.ensureRuntimeCalled {
		t.Fatalf("expected ensureRuntime delegation")
	}
	if _, errResp := mgr.startRuntime(context.Background(), "job-1", preparedRequest{}, &TaskInput{Kind: "image", ID: "image-1"}); errResp != nil {
		t.Fatalf("unexpected startRuntime error: %+v", errResp)
	}
	if !executor.startRuntimeCalled {
		t.Fatalf("expected startRuntime delegation")
	}
	if err := mgr.ensureBaseState(context.Background(), "image-1", "base"); err != nil {
		t.Fatalf("unexpected ensureBaseState error: %v", err)
	}
	if !snapshot.ensureBaseStateCalled {
		t.Fatalf("expected ensureBaseState delegation")
	}
	if _, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1"); errResp != nil {
		t.Fatalf("unexpected invalidateDirtyCachedState error: %+v", errResp)
	}
	if !snapshot.invalidateCalled {
		t.Fatalf("expected invalidateDirtyCachedState delegation")
	}
}
