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
	m        *Manager
	executor taskExecutorAPI
}

type taskExecutor struct {
	m        *Manager
	snapshot snapshotOrchestratorAPI
}

type snapshotOrchestrator struct {
	m *Manager
}

func (m *Manager) runJob(prepared preparedRequest, jobID string) {
	m.coordinator.runJob(prepared, jobID)
}

func (m *Manager) loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse) {
	return m.coordinator.loadOrPlanTasks(ctx, jobID, prepared)
}

func (m *Manager) buildPlan(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlan(ctx, jobID, prepared)
}

func (m *Manager) buildPlanPsql(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlanPsql(prepared)
}

func (m *Manager) buildPlanLiquibase(ctx context.Context, jobID string, prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	return m.coordinator.buildPlanLiquibase(ctx, jobID, prepared)
}

func (m *Manager) planLiquibaseChangesets(ctx context.Context, jobID string, prepared preparedRequest) ([]LiquibaseChangeset, *ErrorResponse) {
	return m.coordinator.planLiquibaseChangesets(ctx, jobID, prepared)
}

func (m *Manager) executeStateTask(ctx context.Context, jobID string, prepared preparedRequest, task taskState) (string, *ErrorResponse) {
	return m.executor.executeStateTask(ctx, jobID, prepared, task)
}

func (m *Manager) executePrepareStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	return m.executor.executePrepareStep(ctx, jobID, prepared, rt, task)
}

func (m *Manager) executePsqlStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) *ErrorResponse {
	return m.executor.executePsqlStep(ctx, jobID, prepared, rt)
}

func (m *Manager) executeLiquibaseStep(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime, task taskState) *ErrorResponse {
	return m.executor.executeLiquibaseStep(ctx, jobID, prepared, rt, task)
}

func (m *Manager) runLiquibaseUpdateSQL(ctx context.Context, jobID string, prepared preparedRequest, rt *jobRuntime) ([]LiquibaseChangeset, *ErrorResponse) {
	return m.executor.runLiquibaseUpdateSQL(ctx, jobID, prepared, rt)
}

func (m *Manager) createInstance(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (*Result, *ErrorResponse) {
	return m.executor.createInstance(ctx, jobID, prepared, stateID)
}

func (m *Manager) ensureRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput, runner *jobRunner) (*jobRuntime, *ErrorResponse) {
	return m.executor.ensureRuntime(ctx, jobID, prepared, input, runner)
}

func (m *Manager) startRuntime(ctx context.Context, jobID string, prepared preparedRequest, input *TaskInput) (*jobRuntime, *ErrorResponse) {
	return m.executor.startRuntime(ctx, jobID, prepared, input)
}

func (m *Manager) ensureBaseState(ctx context.Context, imageID string, baseDir string) error {
	return m.snapshot.ensureBaseState(ctx, imageID, baseDir)
}

func (m *Manager) invalidateDirtyCachedState(ctx context.Context, jobID string, prepared preparedRequest, stateID string) (bool, *ErrorResponse) {
	return m.snapshot.invalidateDirtyCachedState(ctx, jobID, prepared, stateID)
}
