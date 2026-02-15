package prepare

import "context"

type jobCoordinatorAPI interface {
	runJob(prepared preparedRequest, jobID string)
	loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse)
	buildPlan(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse)
	buildPlanPsql(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse)
	buildPlanLiquibase(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse)
	planLiquibaseChangesets(ctx context.Context, jobID string, prepared preparedRequest) ([]LiquibaseChangeset, *ErrorResponse)
}

type taskExecutorAPI interface {
	executeStateTask(ctx context.Context, jobID string, prepared preparedRequest, task taskState) (string, *ErrorResponse)
	executePrepareStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse
	executePsqlStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) *ErrorResponse
	executeLiquibaseStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse
	runLiquibaseUpdateSQL(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) ([]LiquibaseChangeset, *ErrorResponse)
	createInstance(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (*Result, *ErrorResponse)
	ensureRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput, runner *jobRunner) (*jobRuntime, *ErrorResponse)
	startRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput) (*jobRuntime, *ErrorResponse)
}

type snapshotOrchestratorAPI interface {
	ensureBaseState(ctx context.Context, imageID string, baseDir string) error
	invalidateDirtyCachedState(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (bool, *ErrorResponse)
}

type jobCoordinator struct {
	m        *PrepareService
	executor taskExecutorAPI
}

type taskExecutor struct {
	m        *PrepareService
	snapshot snapshotOrchestratorAPI
}

type snapshotOrchestrator struct {
	m *PrepareService
}

func (m *PrepareService) runJob(prepared preparedRequest, jobID string) {
	m.coordinator.runJob(prepared, jobID)
}

func (m *PrepareService) loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse) {
	return m.coordinator.loadOrPlanTasks(ctx, jobID, prepared)
}

func (m *PrepareService) buildPlan(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlan(ctx, jobID, prepared)
}

func (m *PrepareService) buildPlanPsql(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlanPsql(prepared)
}

func (m *PrepareService) buildPlanLiquibase(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlanLiquibase(ctx, jobID, prepared)
}

func (m *PrepareService) planLiquibaseChangesets(ctx context.Context, jobID string, prepared preparedRequest) ([]LiquibaseChangeset, *ErrorResponse) {
	return m.coordinator.planLiquibaseChangesets(ctx, jobID, prepared)
}

func (m *PrepareService) executeStateTask(ctx context.Context, jobID string, prepared preparedRequest, task taskState) (string, *ErrorResponse) {
	return m.executor.executeStateTask(ctx, jobID, prepared, task)
}

func (m *PrepareService) executePrepareStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	return m.executor.executePrepareStep(ctx, jobID, prepared, rt, task)
}

func (m *PrepareService) executePsqlStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) *ErrorResponse {
	return m.executor.executePsqlStep(ctx, jobID, prepared, rt)
}

func (m *PrepareService) executeLiquibaseStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	return m.executor.executeLiquibaseStep(ctx, jobID, prepared, rt, task)
}

func (m *PrepareService) runLiquibaseUpdateSQL(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) ([]LiquibaseChangeset, *ErrorResponse) {
	return m.executor.runLiquibaseUpdateSQL(ctx, jobID, prepared, rt)
}

func (m *PrepareService) createInstance(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (*Result, *ErrorResponse) {
	return m.executor.createInstance(ctx, jobID, prepared, stateID)
}

func (m *PrepareService) ensureRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput, runner *jobRunner) (*jobRuntime, *ErrorResponse) {
	return m.executor.ensureRuntime(ctx, jobID, prepared, input, runner)
}

func (m *PrepareService) startRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput) (*jobRuntime, *ErrorResponse) {
	return m.executor.startRuntime(ctx, jobID, prepared, input)
}

func (m *PrepareService) ensureBaseState(ctx context.Context, imageID string, baseDir string) error {
	return m.snapshot.ensureBaseState(ctx, imageID, baseDir)
}

func (m *PrepareService) invalidateDirtyCachedState(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (bool, *ErrorResponse) {
	return m.snapshot.invalidateDirtyCachedState(ctx, jobID, prepared, stateID)
}
