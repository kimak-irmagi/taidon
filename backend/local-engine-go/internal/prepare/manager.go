package prepare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare/queue"
	"sqlrs/engine/internal/store"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type Options struct {
	Store   store.Store
	Queue   queue.Store
	Version string
	Now     func() time.Time
	IDGen   func() (string, error)
	Async   bool
}

type Manager struct {
	store   store.Store
	queue   queue.Store
	version string
	now     func() time.Time
	idGen   func() (string, error)
	async   bool

	mu      sync.Mutex
	running map[string]*jobRunner
	events  *eventBus
}

type jobRunner struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type preparedRequest struct {
	request        Request
	normalizedArgs []string
	argsNormalized string
	inputHashes    []inputHash
}

func NewManager(opts Options) (*Manager, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if opts.Queue == nil {
		return nil, fmt.Errorf("queue is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	idGen := opts.IDGen
	if idGen == nil {
		idGen = func() (string, error) {
			return randomHex(16)
		}
	}
	return &Manager{
		store:   opts.Store,
		queue:   opts.Queue,
		version: opts.Version,
		now:     now,
		idGen:   idGen,
		async:   opts.Async,
		running: map[string]*jobRunner{},
		events:  newEventBus(),
	}, nil
}

func (m *Manager) Recover(ctx context.Context) error {
	jobs, err := m.queue.ListJobsByStatus(ctx, []string{StatusQueued, StatusRunning})
	if err != nil {
		return err
	}
	for _, job := range jobs {
		prepared, err := m.prepareFromJob(job)
		if err != nil {
			errResp := errorResponse("internal_error", "cannot restore job request", err.Error())
			_ = m.failJob(job.JobID, errResp)
			continue
		}
		if m.async {
			go m.runJob(prepared, job.JobID)
		} else {
			m.runJob(prepared, job.JobID)
		}
	}
	return nil
}

func (m *Manager) Submit(ctx context.Context, req Request) (Accepted, error) {
	prepared, err := m.prepareRequest(req)
	if err != nil {
		return Accepted{}, err
	}
	jobID, err := m.idGen()
	if err != nil {
		return Accepted{}, err
	}
	now := m.now().UTC().Format(time.RFC3339Nano)
	reqJSON, err := json.Marshal(prepared.request)
	if err != nil {
		return Accepted{}, err
	}

	argsNormalized := prepared.argsNormalized
	job := queue.JobRecord{
		JobID:                 jobID,
		Status:                StatusQueued,
		PrepareKind:           prepared.request.PrepareKind,
		ImageID:               prepared.request.ImageID,
		PlanOnly:              prepared.request.PlanOnly,
		SnapshotMode:          "always",
		PrepareArgsNormalized: &argsNormalized,
		RequestJSON:           strPtr(string(reqJSON)),
		CreatedAt:             now,
	}
	if err := m.queue.CreateJob(ctx, job); err != nil {
		return Accepted{}, err
	}
	_ = m.appendEvent(jobID, Event{
		Type:   "status",
		Ts:     now,
		Status: StatusQueued,
	})

	if m.async {
		go m.runJob(prepared, jobID)
	} else {
		m.runJob(prepared, jobID)
	}

	base := "/v1/prepare-jobs/" + jobID
	return Accepted{
		JobID:     jobID,
		StatusURL: base,
		EventsURL: base + "/events",
		Status:    StatusQueued,
	}, nil
}

func (m *Manager) Get(jobID string) (Status, bool) {
	job, ok, err := m.queue.GetJob(context.Background(), jobID)
	if err != nil || !ok {
		return Status{}, false
	}
	tasks, err := m.queue.ListTasks(context.Background(), jobID)
	if err != nil {
		return Status{}, false
	}
	status := Status{
		JobID:                 job.JobID,
		Status:                job.Status,
		PrepareKind:           job.PrepareKind,
		ImageID:               job.ImageID,
		PlanOnly:              job.PlanOnly,
		PrepareArgsNormalized: valueOrEmpty(job.PrepareArgsNormalized),
		CreatedAt:             strPtr(job.CreatedAt),
		StartedAt:             job.StartedAt,
		FinishedAt:            job.FinishedAt,
		Tasks:                 planTasksFromRecords(tasks),
	}
	if job.ResultJSON != nil {
		var result Result
		if err := json.Unmarshal([]byte(*job.ResultJSON), &result); err == nil {
			status.Result = &result
		}
	}
	if job.ErrorJSON != nil {
		var errResp ErrorResponse
		if err := json.Unmarshal([]byte(*job.ErrorJSON), &errResp); err == nil {
			status.Error = &errResp
		}
	}
	return status, true
}

func (m *Manager) ListJobs(jobID string) []JobEntry {
	jobs, err := m.queue.ListJobs(context.Background(), jobID)
	if err != nil {
		return []JobEntry{}
	}
	entries := make([]JobEntry, 0, len(jobs))
	for _, job := range jobs {
		entry := JobEntry{
			JobID:       job.JobID,
			Status:      job.Status,
			PrepareKind: job.PrepareKind,
			ImageID:     job.ImageID,
			PlanOnly:    job.PlanOnly,
			CreatedAt:   strPtr(job.CreatedAt),
			StartedAt:   job.StartedAt,
			FinishedAt:  job.FinishedAt,
		}
		entries = append(entries, entry)
	}
	return entries
}

func (m *Manager) ListTasks(jobID string) []TaskEntry {
	tasks, err := m.queue.ListTasks(context.Background(), jobID)
	if err != nil || len(tasks) == 0 {
		return []TaskEntry{}
	}
	entries := make([]TaskEntry, 0, len(tasks))
	for _, task := range tasks {
		entries = append(entries, taskEntryFromRecord(task))
	}
	return entries
}

func (m *Manager) Delete(jobID string, opts deletion.DeleteOptions) (deletion.DeleteResult, bool) {
	_, ok, err := m.queue.GetJob(context.Background(), jobID)
	if err != nil || !ok {
		return deletion.DeleteResult{}, false
	}
	tasks, err := m.queue.ListTasks(context.Background(), jobID)
	if err != nil {
		return deletion.DeleteResult{}, false
	}

	blocked := hasRunningTasks(tasks)
	node := deletion.DeleteNode{
		Kind: "job",
		ID:   jobID,
	}
	if blocked && !opts.Force {
		node.Blocked = deletion.BlockActiveTasks
		return deletion.DeleteResult{
			DryRun:  opts.DryRun,
			Outcome: deletion.OutcomeBlocked,
			Root:    node,
		}, true
	}

	result := deletion.DeleteResult{
		DryRun:  opts.DryRun,
		Outcome: deletion.OutcomeDeleted,
		Root:    node,
	}
	if opts.DryRun {
		result.Outcome = deletion.OutcomeWouldDelete
		return result, true
	}

	if blocked && opts.Force {
		runner := m.getRunner(jobID)
		if runner != nil {
			runner.cancel()
			<-runner.done
		}
	}

	if err := m.queue.DeleteJob(context.Background(), jobID); err != nil {
		return deletion.DeleteResult{}, false
	}
	return result, true
}

func (m *Manager) EventsSince(jobID string, index int) ([]Event, bool, bool, error) {
	job, ok, err := m.queue.GetJob(context.Background(), jobID)
	if err != nil {
		return nil, false, false, err
	}
	if !ok {
		return nil, false, false, nil
	}
	events, err := m.queue.ListEventsSince(context.Background(), jobID, index)
	if err != nil {
		return nil, true, false, err
	}
	out := make([]Event, 0, len(events))
	for _, event := range events {
		out = append(out, eventFromRecord(event))
	}
	done := job.Status == StatusSucceeded || job.Status == StatusFailed
	return out, true, done, nil
}

func (m *Manager) WaitForEvent(ctx context.Context, jobID string, index int) error {
	ch := m.events.subscribe(jobID)
	defer m.events.unsubscribe(jobID, ch)
	for {
		count, err := m.queue.CountEvents(ctx, jobID)
		if err != nil {
			return err
		}
		if count > index {
			return nil
		}
		job, ok, err := m.queue.GetJob(ctx, jobID)
		if err != nil {
			return err
		}
		if !ok {
			return errJobNotFound
		}
		if job.Status == StatusSucceeded || job.Status == StatusFailed {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

func (m *Manager) prepareFromJob(job queue.JobRecord) (preparedRequest, error) {
	if job.RequestJSON == nil {
		return preparedRequest{}, fmt.Errorf("request_json is empty")
	}
	var req Request
	if err := json.Unmarshal([]byte(*job.RequestJSON), &req); err != nil {
		return preparedRequest{}, err
	}
	return m.prepareRequest(req)
}

func (m *Manager) runJob(prepared preparedRequest, jobID string) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := m.registerRunner(jobID, cancel)
	defer func() {
		close(runner.done)
		m.unregisterRunner(jobID)
	}()

	now := m.now().UTC()
	startedAt := now.Format(time.RFC3339Nano)
	_ = m.queue.UpdateJob(ctx, jobID, queue.JobUpdate{
		Status:    strPtr(StatusRunning),
		StartedAt: &startedAt,
	})
	_ = m.appendEvent(jobID, Event{
		Type:   "status",
		Ts:     startedAt,
		Status: StatusRunning,
	})
	_ = m.queue.UpdateJob(ctx, jobID, queue.JobUpdate{
		PrepareArgsNormalized: &prepared.argsNormalized,
	})

	tasks, stateID, errResp := m.loadOrPlanTasks(ctx, jobID, prepared)
	if errResp != nil {
		_ = m.failJob(jobID, errResp)
		return
	}

	if prepared.request.PlanOnly {
		if err := m.markTasksSucceeded(ctx, jobID, tasks); err != nil {
			_ = m.failJob(jobID, errorResponse("internal_error", "cannot update task status", err.Error()))
			return
		}
		_ = m.succeedPlan(jobID)
		return
	}

	for _, task := range tasks {
		if ctx.Err() != nil {
			_ = m.failJob(jobID, errorResponse("cancelled", "job cancelled", ""))
			return
		}
		if task.Status == StatusSucceeded {
			continue
		}
		if task.Status == StatusFailed {
			_ = m.failJob(jobID, errorResponse("internal_error", "task failed", task.TaskID))
			return
		}
		if err := m.updateTaskStatus(ctx, jobID, task.TaskID, StatusRunning, strPtr(m.now().UTC().Format(time.RFC3339Nano)), nil, nil); err != nil {
			_ = m.failJob(jobID, errorResponse("internal_error", "cannot update task status", err.Error()))
			return
		}
		switch task.Type {
		case "plan":
		case "state_execute":
			if err := m.executeStateTask(ctx, prepared, task); err != nil {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusFailed, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), err)
				_ = m.failJob(jobID, err)
				return
			}
			stateID = task.OutputStateID
		case "prepare_instance":
			result, errResp := m.createInstance(ctx, prepared, stateID)
			if errResp != nil {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusFailed, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), errResp)
				_ = m.failJob(jobID, errResp)
				return
			}
			if err := m.updateTaskStatus(ctx, jobID, task.TaskID, StatusSucceeded, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), nil); err != nil {
				_ = m.failJob(jobID, errorResponse("internal_error", "cannot update task status", err.Error()))
				return
			}
			_ = m.succeed(jobID, *result)
			return
		}
		if err := m.updateTaskStatus(ctx, jobID, task.TaskID, StatusSucceeded, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), nil); err != nil {
			_ = m.failJob(jobID, errorResponse("internal_error", "cannot update task status", err.Error()))
			return
		}
	}

	if stateID == "" {
		_ = m.failJob(jobID, errorResponse("internal_error", "missing output state", ""))
		return
	}
	result, errResp := m.createInstance(ctx, prepared, stateID)
	if errResp != nil {
		_ = m.failJob(jobID, errResp)
		return
	}
	_ = m.succeed(jobID, *result)
}

func (m *Manager) loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse) {
	taskRecords, err := m.queue.ListTasks(ctx, jobID)
	if err != nil {
		return nil, "", errorResponse("internal_error", "cannot load tasks", err.Error())
	}
	if len(taskRecords) == 0 {
		tasks, stateID, errResp := m.buildPlan(prepared)
		if errResp != nil {
			return nil, "", errResp
		}
		records := taskRecordsFromPlan(jobID, tasks)
		if err := m.queue.ReplaceTasks(ctx, jobID, records); err != nil {
			return nil, "", errorResponse("internal_error", "cannot store tasks", err.Error())
		}
		return taskStatesFromPlan(tasks), stateID, nil
	}
	states := taskStatesFromRecords(taskRecords)
	stateID := findOutputStateID(states)
	for i := range states {
		task := &states[i]
		if task.Status != StatusRunning {
			continue
		}
		if task.Type == "state_execute" {
			if exists, err := m.isStateCached(task.OutputStateID); err == nil && exists {
				finishedAt := m.now().UTC().Format(time.RFC3339Nano)
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusSucceeded, nil, &finishedAt, nil)
				task.Status = StatusSucceeded
			} else {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusQueued, nil, nil, nil)
				task.Status = StatusQueued
			}
		}
	}
	return states, stateID, nil
}

func (m *Manager) executeStateTask(ctx context.Context, prepared preparedRequest, task taskState) *ErrorResponse {
	stateID := task.OutputStateID
	if stateID == "" {
		return errorResponse("internal_error", "missing output state", "")
	}
	cached, err := m.isStateCached(stateID)
	if err != nil {
		return errorResponse("internal_error", "cannot check state cache", err.Error())
	}
	if !cached {
		now := m.now().UTC().Format(time.RFC3339Nano)
		var parentID *string
		if task.Input != nil && task.Input.Kind == "state" {
			parentID = &task.Input.ID
		}
		if err := m.store.CreateState(ctx, store.StateCreate{
			StateID:               stateID,
			ParentStateID:         parentID,
			StateFingerprint:      stateID,
			ImageID:               prepared.request.ImageID,
			PrepareKind:           prepared.request.PrepareKind,
			PrepareArgsNormalized: prepared.argsNormalized,
			CreatedAt:             now,
		}); err != nil {
			if errors.Is(err, context.Canceled) {
				return errorResponse("cancelled", "job cancelled", "")
			}
			return errorResponse("internal_error", "cannot store state", err.Error())
		}
	}
	return nil
}

func (m *Manager) createInstance(ctx context.Context, prepared preparedRequest, stateID string) (*Result, *ErrorResponse) {
	instanceID, err := randomHex(16)
	if err != nil {
		return nil, errorResponse("internal_error", "cannot generate instance id", err.Error())
	}
	created := m.now().UTC().Format(time.RFC3339Nano)
	if err := m.store.CreateInstance(ctx, store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    prepared.request.ImageID,
		CreatedAt:  created,
	}); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, errorResponse("cancelled", "job cancelled", "")
		}
		return nil, errorResponse("internal_error", "cannot store instance", err.Error())
	}
	result := Result{
		DSN:                   buildDSN(instanceID),
		InstanceID:            instanceID,
		StateID:               stateID,
		ImageID:               prepared.request.ImageID,
		PrepareKind:           prepared.request.PrepareKind,
		PrepareArgsNormalized: prepared.argsNormalized,
	}
	return &result, nil
}

func (m *Manager) prepareRequest(req Request) (preparedRequest, error) {
	kind := strings.TrimSpace(req.PrepareKind)
	if kind == "" {
		return preparedRequest{}, ValidationError{Code: "invalid_argument", Message: "prepare_kind is required"}
	}
	if kind != "psql" {
		return preparedRequest{}, ValidationError{Code: "invalid_argument", Message: "unsupported prepare_kind", Details: kind}
	}
	imageID := strings.TrimSpace(req.ImageID)
	if imageID == "" {
		return preparedRequest{}, ValidationError{Code: "invalid_argument", Message: "image_id is required"}
	}
	req.PrepareKind = kind
	req.ImageID = imageID
	prepared, err := preparePsqlArgs(req.PsqlArgs, req.Stdin)
	if err != nil {
		return preparedRequest{}, err
	}
	return preparedRequest{
		request:        req,
		normalizedArgs: prepared.normalizedArgs,
		argsNormalized: prepared.argsNormalized,
		inputHashes:    prepared.inputHashes,
	}, nil
}

func (m *Manager) buildPlan(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	taskHash, errResp := m.computeTaskHash(prepared)
	if errResp != nil {
		return nil, "", errResp
	}
	stateID, errResp := m.computeOutputStateID("image", prepared.request.ImageID, taskHash)
	if errResp != nil {
		return nil, "", errResp
	}
	cached, err := m.isStateCached(stateID)
	if err != nil {
		return nil, "", errorResponse("internal_error", "cannot check state cache", err.Error())
	}
	cachedFlag := cached

	tasks := []PlanTask{
		{
			TaskID:      "plan",
			Type:        "plan",
			PlannerKind: prepared.request.PrepareKind,
		},
		{
			TaskID: "execute-0",
			Type:   "state_execute",
			Input: &TaskInput{
				Kind: "image",
				ID:   prepared.request.ImageID,
			},
			TaskHash:      taskHash,
			OutputStateID: stateID,
			Cached:        &cachedFlag,
		},
		{
			TaskID: "prepare-instance",
			Type:   "prepare_instance",
			Input: &TaskInput{
				Kind: "state",
				ID:   stateID,
			},
			InstanceMode: "ephemeral",
		},
	}
	return tasks, stateID, nil
}

func (m *Manager) computeTaskHash(prepared preparedRequest) (string, *ErrorResponse) {
	hasher := newStateHasher()
	hasher.write("prepare_kind", prepared.request.PrepareKind)
	for i, arg := range prepared.normalizedArgs {
		hasher.write(fmt.Sprintf("arg:%d", i), arg)
	}
	for i, input := range prepared.inputHashes {
		hasher.write(fmt.Sprintf("input:%d:%s", i, input.Kind), input.Value)
	}
	hasher.write("engine_version", m.version)
	taskHash := hasher.sum()
	if taskHash == "" {
		return "", errorResponse("internal_error", "cannot compute task hash", "")
	}
	return taskHash, nil
}

func (m *Manager) computeOutputStateID(inputKind, inputID, taskHash string) (string, *ErrorResponse) {
	hasher := newStateHasher()
	hasher.write("input_kind", inputKind)
	hasher.write("input_id", inputID)
	hasher.write("task_hash", taskHash)
	stateID := hasher.sum()
	if stateID == "" {
		return "", errorResponse("internal_error", "cannot compute state id", "")
	}
	return stateID, nil
}

func (m *Manager) isStateCached(stateID string) (bool, error) {
	if strings.TrimSpace(stateID) == "" {
		return false, nil
	}
	_, ok, err := m.store.GetState(context.Background(), stateID)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (m *Manager) markTasksSucceeded(ctx context.Context, jobID string, tasks []taskState) error {
	for _, task := range tasks {
		if err := m.updateTaskStatus(ctx, jobID, task.TaskID, StatusSucceeded, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), nil); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) updateTaskStatus(ctx context.Context, jobID string, taskID string, status string, startedAt *string, finishedAt *string, errResp *ErrorResponse) error {
	var errJSON *string
	if errResp != nil {
		payload, err := json.Marshal(errResp)
		if err != nil {
			return err
		}
		errJSON = strPtr(string(payload))
	}
	update := queue.TaskUpdate{
		Status:     &status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		ErrorJSON:  errJSON,
	}
	if err := m.queue.UpdateTask(ctx, jobID, taskID, update); err != nil {
		return err
	}
	event := Event{
		Type:   "task",
		Ts:     m.now().UTC().Format(time.RFC3339Nano),
		Status: status,
		TaskID: taskID,
	}
	return m.appendEvent(jobID, event)
}

func (m *Manager) succeed(jobID string, result Result) error {
	now := m.now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := m.queue.UpdateJob(context.Background(), jobID, queue.JobUpdate{
		Status:     strPtr(StatusSucceeded),
		FinishedAt: &now,
		ResultJSON: strPtr(string(payload)),
	}); err != nil {
		return err
	}
	if err := m.appendEvent(jobID, Event{
		Type:   "status",
		Ts:     now,
		Status: StatusSucceeded,
	}); err != nil {
		return err
	}
	return m.appendEvent(jobID, Event{
		Type:   "result",
		Ts:     now,
		Result: &result,
	})
}

func (m *Manager) succeedPlan(jobID string) error {
	now := m.now().UTC().Format(time.RFC3339Nano)
	if err := m.queue.UpdateJob(context.Background(), jobID, queue.JobUpdate{
		Status:     strPtr(StatusSucceeded),
		FinishedAt: &now,
	}); err != nil {
		return err
	}
	return m.appendEvent(jobID, Event{
		Type:   "status",
		Ts:     now,
		Status: StatusSucceeded,
	})
}

func (m *Manager) failJob(jobID string, errResp *ErrorResponse) error {
	now := m.now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(errResp)
	if err != nil {
		return err
	}
	if err := m.queue.UpdateJob(context.Background(), jobID, queue.JobUpdate{
		Status:     strPtr(StatusFailed),
		FinishedAt: &now,
		ErrorJSON:  strPtr(string(payload)),
	}); err != nil {
		return err
	}
	if err := m.appendEvent(jobID, Event{
		Type:   "status",
		Ts:     now,
		Status: StatusFailed,
	}); err != nil {
		return err
	}
	return m.appendEvent(jobID, Event{
		Type:  "error",
		Ts:    now,
		Error: errResp,
	})
}

func (m *Manager) appendEvent(jobID string, event Event) error {
	record := eventRecordFromEvent(jobID, event)
	if _, err := m.queue.AppendEvent(context.Background(), record); err != nil {
		return err
	}
	m.events.notify(jobID)
	return nil
}

func (m *Manager) registerRunner(jobID string, cancel context.CancelFunc) *jobRunner {
	runner := &jobRunner{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.mu.Lock()
	m.running[jobID] = runner
	m.mu.Unlock()
	return runner
}

func (m *Manager) unregisterRunner(jobID string) {
	m.mu.Lock()
	delete(m.running, jobID)
	m.mu.Unlock()
}

func (m *Manager) getRunner(jobID string) *jobRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running[jobID]
}

type taskState struct {
	PlanTask
	Status string
}

func taskStatesFromPlan(tasks []PlanTask) []taskState {
	states := make([]taskState, 0, len(tasks))
	for _, task := range tasks {
		states = append(states, taskState{
			PlanTask: task,
			Status:   StatusQueued,
		})
	}
	return states
}

func taskStatesFromRecords(records []queue.TaskRecord) []taskState {
	states := make([]taskState, 0, len(records))
	for _, task := range records {
		states = append(states, taskState{
			PlanTask: planTaskFromRecord(task),
			Status:   task.Status,
		})
	}
	return states
}

func taskRecordsFromPlan(jobID string, tasks []PlanTask) []queue.TaskRecord {
	records := make([]queue.TaskRecord, 0, len(tasks))
	for i, task := range tasks {
		records = append(records, queue.TaskRecord{
			JobID:         jobID,
			TaskID:        task.TaskID,
			Position:      i,
			Type:          task.Type,
			Status:        StatusQueued,
			PlannerKind:   nullableString(task.PlannerKind),
			InputKind:     nullableString(taskInputKind(task.Input)),
			InputID:       nullableString(taskInputID(task.Input)),
			TaskHash:      nullableString(task.TaskHash),
			OutputStateID: nullableString(task.OutputStateID),
			Cached:        task.Cached,
			InstanceMode:  nullableString(task.InstanceMode),
		})
	}
	return records
}

func planTasksFromRecords(records []queue.TaskRecord) []PlanTask {
	tasks := make([]PlanTask, 0, len(records))
	for _, task := range records {
		tasks = append(tasks, planTaskFromRecord(task))
	}
	return tasks
}

func planTaskFromRecord(task queue.TaskRecord) PlanTask {
	var input *TaskInput
	if task.InputKind != nil && task.InputID != nil {
		input = &TaskInput{
			Kind: *task.InputKind,
			ID:   *task.InputID,
		}
	}
	return PlanTask{
		TaskID:        task.TaskID,
		Type:          task.Type,
		PlannerKind:   valueOrEmpty(task.PlannerKind),
		Input:         input,
		TaskHash:      valueOrEmpty(task.TaskHash),
		OutputStateID: valueOrEmpty(task.OutputStateID),
		Cached:        task.Cached,
		InstanceMode:  valueOrEmpty(task.InstanceMode),
	}
}

func taskEntryFromRecord(task queue.TaskRecord) TaskEntry {
	var input *TaskInput
	if task.InputKind != nil && task.InputID != nil {
		input = &TaskInput{
			Kind: *task.InputKind,
			ID:   *task.InputID,
		}
	}
	return TaskEntry{
		TaskID:        task.TaskID,
		JobID:         task.JobID,
		Type:          task.Type,
		Status:        task.Status,
		PlannerKind:   valueOrEmpty(task.PlannerKind),
		Input:         input,
		TaskHash:      valueOrEmpty(task.TaskHash),
		OutputStateID: valueOrEmpty(task.OutputStateID),
		Cached:        task.Cached,
		InstanceMode:  valueOrEmpty(task.InstanceMode),
	}
}

func eventFromRecord(record queue.EventRecord) Event {
	event := Event{
		Type:    record.Type,
		Ts:      record.Ts,
		Status:  valueOrEmpty(record.Status),
		TaskID:  valueOrEmpty(record.TaskID),
		Message: valueOrEmpty(record.Message),
	}
	if record.ResultJSON != nil {
		var result Result
		if err := json.Unmarshal([]byte(*record.ResultJSON), &result); err == nil {
			event.Result = &result
		}
	}
	if record.ErrorJSON != nil {
		var errResp ErrorResponse
		if err := json.Unmarshal([]byte(*record.ErrorJSON), &errResp); err == nil {
			event.Error = &errResp
		}
	}
	return event
}

func eventRecordFromEvent(jobID string, event Event) queue.EventRecord {
	record := queue.EventRecord{
		JobID:   jobID,
		Type:    event.Type,
		Ts:      event.Ts,
		Status:  nullableString(event.Status),
		TaskID:  nullableString(event.TaskID),
		Message: nullableString(event.Message),
	}
	if event.Result != nil {
		if payload, err := json.Marshal(event.Result); err == nil {
			record.ResultJSON = strPtr(string(payload))
		}
	}
	if event.Error != nil {
		if payload, err := json.Marshal(event.Error); err == nil {
			record.ErrorJSON = strPtr(string(payload))
		}
	}
	return record
}

func findOutputStateID(tasks []taskState) string {
	for i := len(tasks) - 1; i >= 0; i-- {
		if tasks[i].Type == "state_execute" && tasks[i].OutputStateID != "" {
			return tasks[i].OutputStateID
		}
	}
	return ""
}

func hasRunningTasks(tasks []queue.TaskRecord) bool {
	for _, task := range tasks {
		if task.Status == StatusRunning {
			return true
		}
	}
	return false
}

func taskInputKind(input *TaskInput) string {
	if input == nil {
		return ""
	}
	return input.Kind
}

func taskInputID(input *TaskInput) string {
	if input == nil {
		return ""
	}
	return input.ID
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func strPtr(value string) *string {
	return &value
}

var errJobNotFound = fmt.Errorf("job not found")

type eventBus struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{
		subs: map[string]map[chan struct{}]struct{}{},
	}
}

func (b *eventBus) subscribe(jobID string) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	if b.subs[jobID] == nil {
		b.subs[jobID] = map[chan struct{}]struct{}{}
	}
	b.subs[jobID][ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) unsubscribe(jobID string, ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[jobID] != nil {
		delete(b.subs[jobID], ch)
		if len(b.subs[jobID]) == 0 {
			delete(b.subs, jobID)
		}
	}
	close(ch)
}

func (b *eventBus) notify(jobID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[jobID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func buildDSN(instanceID string) string {
	return "postgres://sqlrs@local/instance/" + instanceID
}

func formatTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

var randReader = rand.Reader

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := randReader.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
