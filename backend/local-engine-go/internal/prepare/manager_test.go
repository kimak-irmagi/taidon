package prepare

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare/queue"
	"sqlrs/engine/internal/store"
)

type fakeStore struct {
	createStateErr    error
	createInstanceErr error
	getStateErr       error
	statesByID        map[string]store.StateEntry
	states            []store.StateCreate
	instances         []store.InstanceCreate
}

func (f *fakeStore) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	return nil, nil
}

func (f *fakeStore) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	return store.NameEntry{}, false, nil
}

func (f *fakeStore) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	return nil, nil
}

func (f *fakeStore) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	return store.InstanceEntry{}, false, nil
}

func (f *fakeStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	return nil, nil
}

func (f *fakeStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	if f.getStateErr != nil {
		return store.StateEntry{}, false, f.getStateErr
	}
	if f.statesByID == nil {
		return store.StateEntry{}, false, nil
	}
	entry, ok := f.statesByID[stateID]
	return entry, ok, nil
}

func (f *fakeStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	if f.createStateErr != nil {
		return f.createStateErr
	}
	f.states = append(f.states, entry)
	return nil
}

func (f *fakeStore) CreateInstance(ctx context.Context, entry store.InstanceCreate) error {
	if f.createInstanceErr != nil {
		return f.createInstanceErr
	}
	f.instances = append(f.instances, entry)
	return nil
}

func (f *fakeStore) DeleteInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (f *fakeStore) DeleteState(ctx context.Context, stateID string) error {
	return nil
}

func (f *fakeStore) Close() error {
	return nil
}

type blockingStore struct {
	fakeStore
	started chan struct{}
}

func (b *blockingStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-ctx.Done()
	return ctx.Err()
}

type cancelStore struct {
	fakeStore
}

func (c *cancelStore) CreateInstance(ctx context.Context, entry store.InstanceCreate) error {
	return ctx.Err()
}

type cancelStateStore struct {
	fakeStore
}

func (c *cancelStateStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	return ctx.Err()
}

type faultQueueStore struct {
	queue.Store

	createJob        func(context.Context, queue.JobRecord) error
	updateJob        func(context.Context, string, queue.JobUpdate) error
	getJob           func(context.Context, string) (queue.JobRecord, bool, error)
	listJobs         func(context.Context, string) ([]queue.JobRecord, error)
	listJobsByStatus func(context.Context, []string) ([]queue.JobRecord, error)
	deleteJob        func(context.Context, string) error

	replaceTasks func(context.Context, string, []queue.TaskRecord) error
	listTasks    func(context.Context, string) ([]queue.TaskRecord, error)
	updateTask   func(context.Context, string, string, queue.TaskUpdate) error

	appendEvent     func(context.Context, queue.EventRecord) (int64, error)
	listEventsSince func(context.Context, string, int) ([]queue.EventRecord, error)
	countEvents     func(context.Context, string) (int, error)
}

func (f *faultQueueStore) CreateJob(ctx context.Context, job queue.JobRecord) error {
	if f.createJob != nil {
		return f.createJob(ctx, job)
	}
	return f.Store.CreateJob(ctx, job)
}

func (f *faultQueueStore) UpdateJob(ctx context.Context, jobID string, update queue.JobUpdate) error {
	if f.updateJob != nil {
		return f.updateJob(ctx, jobID, update)
	}
	return f.Store.UpdateJob(ctx, jobID, update)
}

func (f *faultQueueStore) GetJob(ctx context.Context, jobID string) (queue.JobRecord, bool, error) {
	if f.getJob != nil {
		return f.getJob(ctx, jobID)
	}
	return f.Store.GetJob(ctx, jobID)
}

func (f *faultQueueStore) ListJobs(ctx context.Context, jobID string) ([]queue.JobRecord, error) {
	if f.listJobs != nil {
		return f.listJobs(ctx, jobID)
	}
	return f.Store.ListJobs(ctx, jobID)
}

func (f *faultQueueStore) ListJobsByStatus(ctx context.Context, statuses []string) ([]queue.JobRecord, error) {
	if f.listJobsByStatus != nil {
		return f.listJobsByStatus(ctx, statuses)
	}
	return f.Store.ListJobsByStatus(ctx, statuses)
}

func (f *faultQueueStore) DeleteJob(ctx context.Context, jobID string) error {
	if f.deleteJob != nil {
		return f.deleteJob(ctx, jobID)
	}
	return f.Store.DeleteJob(ctx, jobID)
}

func (f *faultQueueStore) ReplaceTasks(ctx context.Context, jobID string, tasks []queue.TaskRecord) error {
	if f.replaceTasks != nil {
		return f.replaceTasks(ctx, jobID, tasks)
	}
	return f.Store.ReplaceTasks(ctx, jobID, tasks)
}

func (f *faultQueueStore) ListTasks(ctx context.Context, jobID string) ([]queue.TaskRecord, error) {
	if f.listTasks != nil {
		return f.listTasks(ctx, jobID)
	}
	return f.Store.ListTasks(ctx, jobID)
}

func (f *faultQueueStore) UpdateTask(ctx context.Context, jobID string, taskID string, update queue.TaskUpdate) error {
	if f.updateTask != nil {
		return f.updateTask(ctx, jobID, taskID, update)
	}
	return f.Store.UpdateTask(ctx, jobID, taskID, update)
}

func (f *faultQueueStore) AppendEvent(ctx context.Context, event queue.EventRecord) (int64, error) {
	if f.appendEvent != nil {
		return f.appendEvent(ctx, event)
	}
	return f.Store.AppendEvent(ctx, event)
}

func (f *faultQueueStore) ListEventsSince(ctx context.Context, jobID string, offset int) ([]queue.EventRecord, error) {
	if f.listEventsSince != nil {
		return f.listEventsSince(ctx, jobID, offset)
	}
	return f.Store.ListEventsSince(ctx, jobID, offset)
}

func (f *faultQueueStore) CountEvents(ctx context.Context, jobID string) (int, error) {
	if f.countEvents != nil {
		return f.countEvents(ctx, jobID)
	}
	return f.Store.CountEvents(ctx, jobID)
}

func (f *faultQueueStore) Close() error {
	return f.Store.Close()
}

func TestNewManagerRequiresStore(t *testing.T) {
	queueStore := newQueueStore(t)
	if _, err := NewManager(Options{Queue: queueStore}); err == nil {
		t.Fatalf("expected error when store is nil")
	}
}

func TestNewManagerRequiresQueue(t *testing.T) {
	if _, err := NewManager(Options{Store: &fakeStore{}}); err == nil {
		t.Fatalf("expected error when queue is nil")
	}
}

func TestSubmitRejectsInvalidKind(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	_, err := mgr.Submit(context.Background(), Request{PrepareKind: "", ImageID: "img"})
	expectValidationError(t, err, "prepare_kind is required")

	_, err = mgr.Submit(context.Background(), Request{PrepareKind: "liquibase", ImageID: "img"})
	expectValidationError(t, err, "unsupported prepare_kind")
}

func TestSubmitRejectsMissingImageID(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	_, err := mgr.Submit(context.Background(), Request{PrepareKind: "psql"})
	expectValidationError(t, err, "image_id is required")
}

func TestSubmitIDGenFails(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr, err := NewManager(Options{
		Store:   &fakeStore{},
		Queue:   queueStore,
		Version: "v1",
		IDGen: func() (string, error) {
			return "", errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSubmitCreateJobFails(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		createJob: func(context.Context, queue.JobRecord) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSubmitStoresStateAndInstance(t *testing.T) {
	store := &fakeStore{}
	mgr := newManager(t, store)

	accepted, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if accepted.JobID == "" || accepted.StatusURL == "" || accepted.EventsURL == "" {
		t.Fatalf("unexpected accepted: %+v", accepted)
	}
	if len(store.states) != 1 || len(store.instances) != 1 {
		t.Fatalf("expected state and instance to be stored")
	}

	status, ok := mgr.Get(accepted.JobID)
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusSucceeded || status.Result == nil {
		t.Fatalf("unexpected status: %+v", status)
	}

	tasks := mgr.ListTasks(accepted.JobID)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Status != StatusSucceeded {
			t.Fatalf("expected succeeded tasks, got %+v", tasks)
		}
	}

	events, ok, done, err := mgr.EventsSince(accepted.JobID, 0)
	if err != nil || !ok || !done || len(events) == 0 {
		t.Fatalf("unexpected events: ok=%v done=%v err=%v len=%d", ok, done, err, len(events))
	}
	foundTask := false
	for _, event := range events {
		if event.Type == "task" && event.TaskID != "" {
			foundTask = true
		}
	}
	if !foundTask {
		t.Fatalf("expected task events, got %+v", events)
	}
}

func TestSubmitPlanOnlyBuildsTasks(t *testing.T) {
	store := &fakeStore{}
	mgr := newManager(t, store)

	accepted, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(store.states) != 0 || len(store.instances) != 0 {
		t.Fatalf("expected no state or instance creation")
	}

	status, ok := mgr.Get(accepted.JobID)
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusSucceeded || !status.PlanOnly {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Result != nil {
		t.Fatalf("expected no result for plan-only, got %+v", status.Result)
	}
	if status.PrepareArgsNormalized == "" {
		t.Fatalf("expected normalized args")
	}
	if len(status.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(status.Tasks))
	}

	tasks := mgr.ListTasks(accepted.JobID)
	for _, task := range tasks {
		if task.Status != StatusSucceeded {
			t.Fatalf("expected succeeded tasks, got %+v", tasks)
		}
	}
}

func TestSubmitPlanOnlyCachedFlag(t *testing.T) {
	fake := &fakeStore{statesByID: map[string]store.StateEntry{}}
	mgr := newManager(t, fake)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	taskHash, errResp := mgr.computeTaskHash(prepared)
	if errResp != nil {
		t.Fatalf("computeTaskHash: %+v", errResp)
	}
	stateID, errResp := mgr.computeOutputStateID("image", prepared.request.ImageID, taskHash)
	if errResp != nil {
		t.Fatalf("computeOutputStateID: %+v", errResp)
	}
	fake.statesByID[stateID] = store.StateEntry{StateID: stateID}

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if len(status.Tasks) < 2 || status.Tasks[1].Cached == nil || !*status.Tasks[1].Cached {
		t.Fatalf("expected cached true, got %+v", status.Tasks)
	}
}

func TestListJobsAndTasks(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	_, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	jobs := mgr.ListJobs("")
	if len(jobs) != 1 || jobs[0].JobID != "job-1" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}

	tasks := mgr.ListTasks("job-1")
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestListJobsErrorReturnsEmpty(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		listJobs: func(context.Context, string) ([]queue.JobRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if jobs := mgr.ListJobs(""); len(jobs) != 0 {
		t.Fatalf("expected empty jobs, got %+v", jobs)
	}
}

func TestListTasksErrorReturnsEmpty(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		listTasks: func(context.Context, string) ([]queue.TaskRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if tasks := mgr.ListTasks("job-1"); len(tasks) != 0 {
		t.Fatalf("expected empty tasks, got %+v", tasks)
	}
}

func TestDeleteJobRemovesEntry(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	_, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	result, ok := mgr.Delete("job-1", deletion.DeleteOptions{})
	if !ok || result.Outcome != deletion.OutcomeDeleted {
		t.Fatalf("unexpected delete result: ok=%v result=%+v", ok, result)
	}
	if len(mgr.ListJobs("")) != 0 {
		t.Fatalf("expected jobs to be removed")
	}
}

func TestDeleteJobBlockedWithoutForce(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status := StatusRunning
	if err := queueStore.UpdateTask(context.Background(), "job-1", "plan", queue.TaskUpdate{
		Status: &status,
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	result, ok := mgr.Delete("job-1", deletion.DeleteOptions{})
	if !ok || result.Outcome != deletion.OutcomeBlocked || result.Root.Blocked != deletion.BlockActiveTasks {
		t.Fatalf("unexpected blocked result: ok=%v result=%+v", ok, result)
	}
}

func TestDeleteListTasksError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		listTasks: func(context.Context, string) ([]queue.TaskRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	createJobRecord(t, queueStore, "job-1", Request{PrepareKind: "psql", ImageID: "image-1"}, StatusQueued)

	if _, ok := mgr.Delete("job-1", deletion.DeleteOptions{}); ok {
		t.Fatalf("expected delete to fail")
	}
}

func TestDeleteJobForceCancels(t *testing.T) {
	queueStore := newQueueStore(t)
	blocker := &blockingStore{started: make(chan struct{})}
	mgr := newManagerWithQueue(t, blocker, queueStore)
	mgr.async = true

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case <-blocker.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for CreateState")
	}

	result, ok := mgr.Delete("job-1", deletion.DeleteOptions{Force: true})
	if !ok || result.Outcome != deletion.OutcomeDeleted {
		t.Fatalf("unexpected delete result: ok=%v result=%+v", ok, result)
	}
	if len(mgr.ListJobs("")) != 0 {
		t.Fatalf("expected job removed")
	}
}

func TestRecoverQueuedJob(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	reqJSON, err := jsonMarshal(req)
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusQueued,
		PrepareKind: "psql",
		ImageID:     "image-1",
		PlanOnly:    false,
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	store := &fakeStore{}
	mgr := newManagerWithQueue(t, store, queueStore)
	mgr.async = false

	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusSucceeded || status.Result == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRecoverListJobsError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		listJobsByStatus: func(context.Context, []string) ([]queue.JobRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if err := mgr.Recover(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRecoverMissingRequestMarksFailed(t *testing.T) {
	queueStore := newQueueStore(t)
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusQueued,
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed || status.Error == nil {
		t.Fatalf("expected failed job, got %+v", status)
	}
}

func TestPrepareFromJobInvalidJSON(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if _, err := mgr.prepareFromJob(queue.JobRecord{RequestJSON: strPtr("{")}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadOrPlanTasksPromotesRunningTasks(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	reqJSON, err := jsonMarshal(req)
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusRunning,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	store := &fakeStore{statesByID: map[string]store.StateEntry{"state-1": {StateID: "state-1"}}}
	mgr := newManagerWithQueue(t, store, queueStore)
	tasks := []queue.TaskRecord{
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      0,
			Type:          "state_execute",
			Status:        StatusRunning,
			OutputStateID: strPtr("state-1"),
		},
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", tasks); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	prepared, err := mgr.prepareRequest(req)
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	states, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared)
	if errResp != nil {
		t.Fatalf("loadOrPlanTasks: %+v", errResp)
	}
	if len(states) != 1 || states[0].Status != StatusSucceeded {
		t.Fatalf("unexpected task states: %+v", states)
	}
}

func TestLoadOrPlanTasksListError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		listTasks: func(context.Context, string) ([]queue.TaskRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if _, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared); errResp == nil {
		t.Fatalf("expected error response")
	}
}

func TestLoadOrPlanTasksReplaceError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		replaceTasks: func(context.Context, string, []queue.TaskRecord) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if _, _, errResp := mgr.loadOrPlanTasks(context.Background(), "job-1", prepared); errResp == nil {
		t.Fatalf("expected error response")
	}
}

func TestExecuteStateTaskCachedSkipsCreate(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{"state-1": {StateID: "state-1"}}}
	mgr := newManager(t, store)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: "state-1",
		},
	}
	if errResp := mgr.executeStateTask(context.Background(), prepared, task); errResp != nil {
		t.Fatalf("executeStateTask: %+v", errResp)
	}
	if len(store.states) != 0 {
		t.Fatalf("expected no state creation")
	}
}

func TestExecuteStateTaskCreateStateError(t *testing.T) {
	store := &fakeStore{createStateErr: errors.New("boom")}
	mgr := newManager(t, store)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: "state-1",
		},
	}
	errResp := mgr.executeStateTask(context.Background(), prepared, task)
	if errResp == nil || errResp.Code != "internal_error" {
		t.Fatalf("expected internal error, got %+v", errResp)
	}
}

func TestExecuteStateTaskCancelled(t *testing.T) {
	store := &cancelStateStore{}
	mgr := newManager(t, store)

	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	task := taskState{
		PlanTask: PlanTask{
			TaskID:        "execute-0",
			Type:          "state_execute",
			OutputStateID: "state-1",
			Input:         &TaskInput{Kind: "state", ID: "parent-1"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errResp := mgr.executeStateTask(ctx, prepared, task)
	if errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestCreateInstanceErrors(t *testing.T) {
	store := &fakeStore{createInstanceErr: errors.New("boom")}
	mgr := newManager(t, store)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if _, errResp := mgr.createInstance(context.Background(), prepared, "state-1"); errResp == nil || errResp.Code != "internal_error" {
		t.Fatalf("expected internal error, got %+v", errResp)
	}
}

func TestCreateInstanceCancelled(t *testing.T) {
	store := &cancelStore{}
	mgr := newManager(t, store)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, errResp := mgr.createInstance(ctx, prepared, "state-1"); errResp == nil || errResp.Code != "cancelled" {
		t.Fatalf("expected cancelled error, got %+v", errResp)
	}
}

func TestEventFromRecordParsesPayloads(t *testing.T) {
	result := Result{DSN: "dsn", InstanceID: "i", StateID: "s", ImageID: "img", PrepareKind: "psql", PrepareArgsNormalized: "args"}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	errResp := ErrorResponse{Code: "boom", Message: "fail"}
	errJSON, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	event := eventFromRecord(queue.EventRecord{
		Type:       "result",
		Ts:         "2026-01-19T00:00:00Z",
		ResultJSON: strPtr(string(resultJSON)),
		ErrorJSON:  strPtr(string(errJSON)),
	})
	if event.Result == nil || event.Result.DSN != "dsn" {
		t.Fatalf("unexpected result: %+v", event.Result)
	}
	if event.Error == nil || event.Error.Code != "boom" {
		t.Fatalf("unexpected error: %+v", event.Error)
	}
}

func TestFindOutputStateID(t *testing.T) {
	stateID := findOutputStateID([]taskState{
		{PlanTask: PlanTask{Type: "plan"}},
		{PlanTask: PlanTask{Type: "state_execute", OutputStateID: "state-1"}},
	})
	if stateID != "state-1" {
		t.Fatalf("unexpected state id: %s", stateID)
	}
}

func TestWaitForEvent(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := mgr.WaitForEvent(context.Background(), "job-1", 0); err != nil {
		t.Fatalf("WaitForEvent: %v", err)
	}

	count, err := queueStore.CountEvents(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("CountEvents: %v", err)
	}
	if err := mgr.WaitForEvent(context.Background(), "job-1", count); err != nil {
		t.Fatalf("WaitForEvent after done: %v", err)
	}
}

func TestWaitForEventDoneNoEvents(t *testing.T) {
	queueStore := newQueueStore(t)
	reqJSON, err := jsonMarshal(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusSucceeded,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	if err := mgr.WaitForEvent(context.Background(), "job-1", 0); err != nil {
		t.Fatalf("WaitForEvent: %v", err)
	}
}

func TestWaitForEventCountError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		countEvents: func(context.Context, string) (int, error) {
			return 0, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if err := mgr.WaitForEvent(context.Background(), "job-1", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForEventNotifies(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- mgr.WaitForEvent(ctx, "job-1", 0)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		mgr.events.mu.Lock()
		_, ok := mgr.events.subs["job-1"]
		mgr.events.mu.Unlock()
		if ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for subscription")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := mgr.appendEvent("job-1", Event{
		Type:   "status",
		Ts:     time.Now().UTC().Format(time.RFC3339Nano),
		Status: StatusRunning,
	}); err != nil {
		t.Fatalf("appendEvent: %v", err)
	}
	if err := <-waitErr; err != nil {
		t.Fatalf("WaitForEvent: %v", err)
	}
}

func TestWaitForEventGetJobError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		countEvents: func(context.Context, string) (int, error) {
			return 0, nil
		},
		getJob: func(context.Context, string) (queue.JobRecord, bool, error) {
			return queue.JobRecord{}, false, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if err := mgr.WaitForEvent(context.Background(), "job-1", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForEventCancel(t *testing.T) {
	queueStore := newQueueStore(t)
	reqJSON, err := jsonMarshal(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusRunning,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mgr.WaitForEvent(ctx, "job-1", 0); err == nil {
		t.Fatalf("expected cancel error")
	}
}

func TestEventBusNotify(t *testing.T) {
	bus := newEventBus()
	ch := bus.subscribe("job-1")
	bus.notify("job-1")
	select {
	case <-ch:
	default:
		t.Fatalf("expected notification")
	}
	bus.unsubscribe("job-1", ch)
}

func TestEventsSinceUnknown(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	events, ok, done, err := mgr.EventsSince("missing", 0)
	if err != nil || ok || done || events != nil {
		t.Fatalf("expected missing job, got ok=%v done=%v err=%v events=%v", ok, done, err, events)
	}
}

func TestEventsSinceListError(t *testing.T) {
	queueStore := newQueueStore(t)
	reqJSON, err := jsonMarshal(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusQueued,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	faulty := &faultQueueStore{
		Store: queueStore,
		listEventsSince: func(context.Context, string, int) ([]queue.EventRecord, error) {
			return nil, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)

	if _, ok, done, err := mgr.EventsSince("job-1", 0); err == nil || !ok || done {
		t.Fatalf("expected list error, ok=%v done=%v err=%v", ok, done, err)
	}
}

func TestGetUnknownJob(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if _, ok := mgr.Get("missing"); ok {
		t.Fatalf("expected missing job")
	}
}

func TestGetIncludesErrorPayload(t *testing.T) {
	queueStore := newQueueStore(t)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	reqJSON, err := jsonMarshal(req)
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	errResp := ErrorResponse{Code: "boom", Message: "fail"}
	errJSON, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusFailed,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		ErrorJSON:   strPtr(string(errJSON)),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	status, ok := mgr.Get("job-1")
	if !ok || status.Error == nil || status.Error.Code != "boom" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestPrepareRequestPsqlError(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if _, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-f"},
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsStateCachedEmpty(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	cached, err := mgr.isStateCached("")
	if err != nil || cached {
		t.Fatalf("expected empty cache result, cached=%v err=%v", cached, err)
	}
}

func TestIsStateCachedError(t *testing.T) {
	store := &fakeStore{getStateErr: errors.New("boom")}
	mgr := newManager(t, store)

	if _, err := mgr.isStateCached("state-1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRandomHex(t *testing.T) {
	value, err := randomHex(4)
	if err != nil {
		t.Fatalf("randomHex: %v", err)
	}
	if len(value) != 8 {
		t.Fatalf("expected 8 chars, got %d", len(value))
	}
	if _, err := hex.DecodeString(value); err != nil {
		t.Fatalf("expected hex string, got %q", value)
	}
}

func TestRandomHexError(t *testing.T) {
	prevReader := randReader
	randReader = errorReader{}
	t.Cleanup(func() { randReader = prevReader })

	if _, err := randomHex(4); err == nil {
		t.Fatalf("expected error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

func TestNewManagerDefaultIDGen(t *testing.T) {
	queueStore := newQueueStore(t)
	mgr, err := NewManager(Options{
		Store: &fakeStore{},
		Queue: queueStore,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr.idGen == nil {
		t.Fatalf("expected id generator")
	}
	if mgr.now == nil {
		t.Fatalf("expected now function")
	}
}

func TestSubmitAsyncRunsJob(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	mgr.async = true

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status, ok := mgr.Get("job-1")
		if ok && status.Status != StatusQueued {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job did not start")
}

func TestDeleteUnknownJob(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if _, ok := mgr.Delete("missing", deletion.DeleteOptions{}); ok {
		t.Fatalf("expected delete to miss")
	}
}

func TestDeleteJobDryRun(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	result, ok := mgr.Delete("job-1", deletion.DeleteOptions{DryRun: true})
	if !ok || result.Outcome != deletion.OutcomeWouldDelete {
		t.Fatalf("unexpected delete result: ok=%v result=%+v", ok, result)
	}
}

func TestListJobsMissingJob(t *testing.T) {
	mgr := newManager(t, &fakeStore{})

	if jobs := mgr.ListJobs("missing"); len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %+v", jobs)
	}
}

func TestSubmitCreateStateFails(t *testing.T) {
	store := &fakeStore{createStateErr: errors.New("boom")}
	mgr := newManager(t, store)

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed || status.Error == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSubmitCreateInstanceFails(t *testing.T) {
	store := &fakeStore{createInstanceErr: errors.New("boom")}
	mgr := newManager(t, store)

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed || status.Error == nil {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSubmitInstanceIDFails(t *testing.T) {
	store := &fakeStore{}
	mgr := newManager(t, store)

	prevReader := randReader
	randReader = errorReader{}
	t.Cleanup(func() { randReader = prevReader })

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed || status.Error == nil || status.Error.Message != "cannot generate instance id" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRecoverWithFailedTask(t *testing.T) {
	queueStore := newQueueStore(t)
	reqJSON, err := jsonMarshal(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusRunning,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := queueStore.ReplaceTasks(context.Background(), "job-1", []queue.TaskRecord{
		{
			JobID:    "job-1",
			TaskID:   "execute-0",
			Position: 0,
			Type:     "state_execute",
			Status:   StatusFailed,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRecoverMissingStateIDFailsJob(t *testing.T) {
	queueStore := newQueueStore(t)
	reqJSON, err := jsonMarshal(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       "job-1",
		Status:      StatusRunning,
		PrepareKind: "psql",
		ImageID:     "image-1",
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
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

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if err := mgr.Recover(context.Background()); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	status, ok := mgr.Get("job-1")
	if !ok || status.Status != StatusFailed {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestBuildPlanCacheError(t *testing.T) {
	store := &fakeStore{getStateErr: errors.New("boom")}
	mgr := newManager(t, store)
	prepared, err := mgr.prepareRequest(Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("prepareRequest: %v", err)
	}
	if _, _, errResp := mgr.buildPlan(prepared); errResp == nil {
		t.Fatalf("expected error")
	}
}

func TestRunJobTaskFailed(t *testing.T) {
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
			TaskID:   "execute-0",
			Position: 0,
			Type:     "state_execute",
			Status:   StatusFailed,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
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

func TestRunJobMissingOutputState(t *testing.T) {
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

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
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

func TestRunJobPlanOnlyMarkTasksFail(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateTask: func(context.Context, string, string, queue.TaskUpdate) error {
			return errors.New("boom")
		},
	}
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)

	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
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

func TestRunJobUpdateTaskStatusFails(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateTask: func(context.Context, string, string, queue.TaskUpdate) error {
			return errors.New("boom")
		},
	}
	req := Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}
	createJobRecord(t, queueStore, "job-1", req, StatusQueued)

	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
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

func TestRunJobStateExecuteMissingOutputState(t *testing.T) {
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
			TaskID:   "execute-0",
			Position: 0,
			Type:     "state_execute",
			Status:   StatusQueued,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
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

func TestRunJobSkipsSucceededTask(t *testing.T) {
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
		{
			JobID:    "job-1",
			TaskID:   "prepare-instance",
			Position: 2,
			Type:     "prepare_instance",
			Status:   StatusQueued,
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
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

func TestRunJobCreateInstanceAfterLoopFails(t *testing.T) {
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
		{
			JobID:         "job-1",
			TaskID:        "execute-0",
			Position:      1,
			Type:          "state_execute",
			Status:        StatusQueued,
			OutputStateID: strPtr("state-1"),
		},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}

	store := &fakeStore{createInstanceErr: errors.New("boom")}
	mgr := newManagerWithQueue(t, store, queueStore)
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

func TestRunJobPrepareInstanceUpdateTaskError(t *testing.T) {
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
	updateCalls := 0
	faulty := &faultQueueStore{
		Store: queueStore,
		updateTask: func(ctx context.Context, jobID string, taskID string, update queue.TaskUpdate) error {
			updateCalls++
			if updateCalls == 4 {
				return errors.New("boom")
			}
			return queueStore.UpdateTask(ctx, jobID, taskID, update)
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
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

func TestWaitForEventNotFound(t *testing.T) {
	mgr := newManager(t, &fakeStore{})
	if err := mgr.WaitForEvent(context.Background(), "missing", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEventsSinceError(t *testing.T) {
	queueStore := newQueueStore(t)
	if err := queueStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, queueStore)
	if _, _, _, err := mgr.EventsSince("job-1", 0); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFormatTimeHelpers(t *testing.T) {
	if formatTime(time.Time{}) != nil {
		t.Fatalf("expected nil for zero time")
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if formatTime(now) == nil {
		t.Fatalf("expected formatted time")
	}
	if formatTimePtr(nil) != nil {
		t.Fatalf("expected nil for nil pointer")
	}
	if formatTimePtr(&now) == nil {
		t.Fatalf("expected formatted time for pointer")
	}
}

func TestSucceedUpdateJobError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)

	result := Result{
		DSN:                   "dsn",
		InstanceID:            "inst",
		StateID:               "state-1",
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "args",
	}
	if err := mgr.succeed("job-1", result); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSucceedAppendEventError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		appendEvent: func(context.Context, queue.EventRecord) (int64, error) {
			return 0, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)

	result := Result{
		DSN:                   "dsn",
		InstanceID:            "inst",
		StateID:               "state-1",
		ImageID:               "image-1",
		PrepareKind:           "psql",
		PrepareArgsNormalized: "args",
	}
	if err := mgr.succeed("job-1", result); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSucceedPlanUpdateJobError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)

	if err := mgr.succeedPlan("job-1"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFailJobAppendEventError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		appendEvent: func(context.Context, queue.EventRecord) (int64, error) {
			return 0, errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)

	if err := mgr.failJob("job-1", errorResponse("boom", "fail", "")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFailJobUpdateJobError(t *testing.T) {
	queueStore := newQueueStore(t)
	faulty := &faultQueueStore{
		Store: queueStore,
		updateJob: func(context.Context, string, queue.JobUpdate) error {
			return errors.New("boom")
		},
	}
	mgr := newManagerWithQueue(t, &fakeStore{}, faulty)
	req := Request{PrepareKind: "psql", ImageID: "image-1"}
	createJobRecord(t, queueStore, "job-1", req, StatusRunning)

	if err := mgr.failJob("job-1", errorResponse("boom", "fail", "")); err == nil {
		t.Fatalf("expected error")
	}
}

func newManager(t *testing.T, store store.Store) *Manager {
	t.Helper()
	return newManagerWithQueue(t, store, newQueueStore(t))
}

func newManagerWithQueue(t *testing.T, store store.Store, queueStore queue.Store) *Manager {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mgr, err := NewManager(Options{
		Store:   store,
		Queue:   queueStore,
		Version: "v1",
		Now:     func() time.Time { return now },
		IDGen:   func() (string, error) { return "job-1", nil },
		Async:   false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

func newQueueStore(t *testing.T) queue.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := queue.Open(path)
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func jsonMarshal(req Request) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func createJobRecord(t *testing.T, queueStore queue.Store, jobID string, req Request, status string) {
	t.Helper()
	reqJSON, err := jsonMarshal(req)
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if err := queueStore.CreateJob(context.Background(), queue.JobRecord{
		JobID:       jobID,
		Status:      status,
		PrepareKind: req.PrepareKind,
		ImageID:     req.ImageID,
		PlanOnly:    req.PlanOnly,
		RequestJSON: &reqJSON,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
}
