package prepare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/prepare/queue"
	engineRuntime "sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
)

func TestLoadOrPlanTasksRequeuesRunningStateWhenCacheMissing(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      0,
			Type:          "state_execute",
			Status:        StatusRunning,
			OutputStateID: strPtr("state-missing"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{statesByID: map[string]store.StateEntry{}}, queueStore)
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	states, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared)
	if errResp != nil {
		t.Fatalf("loadOrPlanTasks: %+v", errResp)
	}
	if len(states) != 1 || states[0].Status != StatusQueued {
		t.Fatalf("expected running state task to be requeued, got %+v", states)
	}
}

func TestUpdateJobSignatureReturnsComputeError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}

	errResp := mgr.updateJobSignature(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected compute signature error, got %+v", errResp)
	}
}

func TestUpdateJobSignatureFromPlanReturnsComputeError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}

	errResp := mgr.updateJobSignatureFromPlan(context.Background(), "job-1", prepared, nil)
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected compute signature error, got %+v", errResp)
	}
}

func TestComputeJobSignatureReturnsTaskHashError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "psql",
			ImageID:     "image-1@sha256:abc",
		},
		psqlInputs: []psqlInput{
			{kind: "file", value: filepath.Join(t.TempDir(), "missing.sql")},
		},
	}

	_, errResp := mgr.computeJobSignature(prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot compute psql content hash") {
		t.Fatalf("expected task hash error, got %+v", errResp)
	}
}

func TestTrimCompletedJobsForJobHandlesLookupError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{}, false, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	mgr.trimCompletedJobsForJob(context.Background(), "job-1")
}

func TestPrepareRequestLiquibaseArgsError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	_, err := mgr.prepareRequest(Request{
		PrepareKind:   "lb",
		ImageID:       "image-1",
		LiquibaseArgs: []string{"update", "--changelog-file"},
	})
	if err == nil {
		t.Fatalf("expected liquibase args error")
	}
}

func TestBuildPlanPsqlReturnsTaskHashError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "psql",
			ImageID:     "image-1",
		},
		psqlInputs: []psqlInput{
			{kind: "file", value: filepath.Join(t.TempDir(), "missing.sql")},
		},
	}
	_, _, errResp := mgr.buildPlanPsql(prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot compute psql content hash") {
		t.Fatalf("expected task hash error, got %+v", errResp)
	}
}

func TestBuildPlanPsqlRequiresResolvedImage(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "psql",
		},
		psqlInputs: []psqlInput{
			{kind: "command", value: "select 1"},
		},
	}
	_, _, errResp := mgr.buildPlanPsql(prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected resolved image error, got %+v", errResp)
	}
}

func TestBuildPlanLiquibaseRequiresResolvedImage(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "lb",
		},
	}
	_, _, errResp := mgr.buildPlanLiquibase(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected resolved image error, got %+v", errResp)
	}
}

func TestPlanLiquibaseChangesetsRequiresRunner(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	mgr.liquibase = nil
	prepared := preparedRequest{request: Request{PrepareKind: "lb", ImageID: "image-1@sha256:abc"}}

	_, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "liquibase runner is required") {
		t.Fatalf("expected runner required error, got %+v", errResp)
	}
}

func TestPlanLiquibaseChangesetsRequiresResolvedImage(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{liquibase: &fakeLiquibaseRunner{}})
	prepared := preparedRequest{request: Request{PrepareKind: "lb"}}

	_, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected resolved image error, got %+v", errResp)
	}
}

func TestPlanLiquibaseChangesetsReturnsEnsureRuntimeError(t *testing.T) {
	rt := &fakeRuntime{startErr: errors.New("boom")}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		runtime:   rt,
		liquibase: &fakeLiquibaseRunner{},
	})
	runner := mgr.registerRunner("job-1", func() {})
	defer mgr.unregisterRunner("job-1")
	close(runner.done)

	prepared := preparedRequest{request: Request{PrepareKind: "lb", ImageID: "image-1@sha256:abc"}}
	_, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot start runtime") {
		t.Fatalf("expected runtime error, got %+v", errResp)
	}
}

func TestPlanLiquibaseChangesetsReturnsStartRuntimeError(t *testing.T) {
	rt := &fakeRuntime{startErr: errors.New("boom")}
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		runtime:   rt,
		liquibase: &fakeLiquibaseRunner{},
	})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "lb",
			ImageID:     "image-1@sha256:abc",
			PlanOnly:    true,
		},
	}
	_, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot start runtime") {
		t.Fatalf("expected runtime error, got %+v", errResp)
	}
}

func TestPlanLiquibaseChangesetsReturnsLockError(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "changelog.sql")
	if err := os.WriteFile(lockPath, []byte("select 1;"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	prev := lockFileSharedFn
	lockFileSharedFn = func(*os.File) error {
		return errors.New("lock failed")
	}
	t.Cleanup(func() { lockFileSharedFn = prev })

	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		runtime:   &fakeRuntime{},
		liquibase: &fakeLiquibaseRunner{},
	})
	prepared := preparedRequest{
		request: Request{
			PrepareKind: "lb",
			ImageID:     "image-1@sha256:abc",
			PlanOnly:    true,
		},
		liquibaseLockPaths: []string{lockPath},
	}
	_, errResp := mgr.planLiquibaseChangesets(context.Background(), "job-1", prepared)
	if errResp == nil || !strings.Contains(errResp.Message, "cannot lock liquibase inputs") {
		t.Fatalf("expected lock error, got %+v", errResp)
	}
}

func TestRunLiquibaseUpdateSQLErrorFallsBackToRunnerError(t *testing.T) {
	mgr := newManagerWithDeps(t, &fakeStore{}, newQueueStore(t), &testDeps{
		liquibase: &fakeLiquibaseRunner{err: errors.New("boom")},
	})
	prepared := preparedRequest{
		request:        Request{PrepareKind: "lb"},
		normalizedArgs: []string{"update"},
	}
	rt := &jobRuntime{instance: engineRuntime.Instance{Host: "127.0.0.1", Port: 5432}}

	_, errResp := mgr.runLiquibaseUpdateSQL(context.Background(), "job-1", prepared, rt)
	if errResp == nil || !strings.Contains(errResp.Message, "liquibase execution failed") || !strings.Contains(errResp.Details, "boom") {
		t.Fatalf("expected liquibase execution fallback error, got %+v", errResp)
	}
}

func TestRunJobResolveImageTaskFailure(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "resolve-image",
			Position: 0,
			Type:     "resolve_image",
			Status:   StatusQueued,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	mgr := newManagerWithDeps(t, &fakeStore{}, queueStore, &testDeps{runtime: &fakeRuntime{resolveErr: errors.New("boom")}})
	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	mgr.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRunJobPrepareInstancePostUpdateFails(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      0,
			Type:          "state_execute",
			Status:        StatusSucceeded,
			OutputStateID: strPtr("state-1"),
		},
		{
			JobID:    "job-1",
			TaskID:   "prepare-instance",
			Position: 1,
			Type:     "prepare_instance",
			Status:   StatusQueued,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	wrapped := &faultQueueStore{
		Store: queueStore,
		updateTask: func(ctx context.Context, jobID string, taskID string, update queue.TaskUpdate) error {
			if taskID == "prepare-instance" && update.Status != nil && *update.Status == StatusSucceeded {
				return errors.New("boom")
			}
			return queueStore.UpdateTask(ctx, jobID, taskID, update)
		},
	}
	stateStore := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithQueue(t, stateStore, wrapped)
	writeStateSnapshotForRuntime(t, mgr, "image-1", "state-1")

	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	mgr.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRunJobTaskPostSuccessUpdateFails(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "plan",
			Position: 0,
			Type:     "plan",
			Status:   StatusQueued,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	wrapped := &faultQueueStore{
		Store: queueStore,
		updateTask: func(ctx context.Context, jobID string, taskID string, update queue.TaskUpdate) error {
			if taskID == "plan" && update.Status != nil && *update.Status == StatusSucceeded {
				return errors.New("boom")
			}
			return queueStore.UpdateTask(ctx, jobID, taskID, update)
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, wrapped)

	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	mgr.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRunJobCreateInstanceAfterLoopSucceeds(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "plan",
			Position: 0,
			Type:     "plan",
			Status:   StatusSucceeded,
		},
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      1,
			Type:          "state_execute",
			Status:        StatusSucceeded,
			OutputStateID: strPtr("state-1"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	stateStore := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithQueue(t, stateStore, queueStore)
	writeStateSnapshotForRuntime(t, mgr, "image-1", "state-1")

	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	mgr.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusSucceeded || status.Result == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRunJobCreateInstanceAfterLoopFailsWithStatePresent(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "plan",
			Position: 0,
			Type:     "plan",
			Status:   StatusSucceeded,
		},
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      1,
			Type:          "state_execute",
			Status:        StatusSucceeded,
			OutputStateID: strPtr("state-1"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	stateStore := &fakeStore{
		createInstanceErr: errors.New("boom"),
		statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1"},
		},
	}
	mgr := newManagerWithQueue(t, stateStore, queueStore)
	writeStateSnapshotForRuntime(t, mgr, "image-1", "state-1")

	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	mgr.runJob(prepared, "job-1")

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestFailJobAllowsNilErrorResponse(t *testing.T) {
	queueStore := newQueueStore(t)
	createJobRecord(t, queueStore, "job-1", Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}, StatusRunning)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	if err := mgr.failJob("job-1", nil); err != nil {
		t.Fatalf("failJob: %v", err)
	}
	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestAppendLogLinesIgnoresWhitespaceContent(t *testing.T) {
	queueStore := newQueueStore(t)
	createJobRecord(t, queueStore, "job-1", Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}, StatusRunning)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	mgr.appendLogLines("job-1", "docker", " \n \r\n ")
	events, ok, _, err := mgr.EventsSince("job-1", 0)
	if err != nil || !ok {
		t.Fatalf("EventsSince: ok=%v err=%v", ok, err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no log events for empty content, got %+v", events)
	}
}

func TestEventBusNotifySkipsWhenChannelIsFull(t *testing.T) {
	bus := newEventBus()
	ch := bus.subscribe("job-1")
	defer bus.unsubscribe("job-1", ch)

	ch <- struct{}{}
	done := make(chan struct{})
	go func() {
		bus.notify("job-1")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("notify blocked on full channel")
	}
	if len(ch) != 1 {
		t.Fatalf("expected channel to stay full, got len=%d", len(ch))
	}
}

func writeStateSnapshotForRuntime(t *testing.T, mgr *PrepareService, imageID string, stateID string) {
	t.Helper()
	paths, err := resolveStatePaths(mgr.stateStoreRoot, imageID, stateID, mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "PG_VERSION"), []byte("17"), 0o600); err != nil {
		t.Fatalf("write PG_VERSION: %v", err)
	}
}
