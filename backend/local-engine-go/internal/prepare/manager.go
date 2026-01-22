package prepare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sqlrs/engine/internal/config"
	"sqlrs/engine/internal/dbms"
	"sqlrs/engine/internal/deletion"
	"sqlrs/engine/internal/prepare/queue"
	"sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/snapshot"
	"sqlrs/engine/internal/store"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type Options struct {
	Store          store.Store
	Queue          queue.Store
	Runtime        runtime.Runtime
	Snapshot       snapshot.Manager
	DBMS           dbms.Connector
	StateStoreRoot string
	Config         config.Store
	Psql           psqlRunner
	Version        string
	Now            func() time.Time
	IDGen          func() (string, error)
	Async          bool
}

type Manager struct {
	store          store.Store
	queue          queue.Store
	runtime        runtime.Runtime
	snapshot       snapshot.Manager
	dbms           dbms.Connector
	stateStoreRoot string
	config         config.Store
	psql           psqlRunner
	version        string
	now            func() time.Time
	idGen          func() (string, error)
	async          bool

	mu      sync.Mutex
	running map[string]*jobRunner
	events  *eventBus
}

type jobRunner struct {
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	rt     *jobRuntime
}

type jobRuntime struct {
	instance    runtime.Instance
	dataDir     string
	runtimeDir  string
	cleanup     func() error
	scriptMount *scriptMount
}

type preparedRequest struct {
	request         Request
	normalizedArgs  []string
	argsNormalized  string
	inputHashes     []inputHash
	filePaths       []string
	resolvedImageID string
}

func NewManager(opts Options) (*Manager, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if opts.Queue == nil {
		return nil, fmt.Errorf("queue is required")
	}
	if opts.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	if opts.Snapshot == nil {
		return nil, fmt.Errorf("snapshot manager is required")
	}
	if opts.DBMS == nil {
		return nil, fmt.Errorf("dbms connector is required")
	}
	if strings.TrimSpace(opts.StateStoreRoot) == "" {
		return nil, fmt.Errorf("state store root is required")
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
	psql := opts.Psql
	if psql == nil {
		psql = containerPsqlRunner{runtime: opts.Runtime}
	}
	return &Manager{
		store:          opts.Store,
		queue:          opts.Queue,
		runtime:        opts.Runtime,
		snapshot:       opts.Snapshot,
		dbms:           opts.DBMS,
		stateStoreRoot: opts.StateStoreRoot,
		config:         opts.Config,
		psql:           psql,
		version:        opts.Version,
		now:            now,
		idGen:          idGen,
		async:          opts.Async,
		running:        map[string]*jobRunner{},
		events:         newEventBus(),
	}, nil
}

func (m *Manager) Recover(ctx context.Context) error {
	jobs, err := m.queue.ListJobsByStatus(ctx, []string{StatusQueued, StatusRunning})
	if err != nil {
		return err
	}
	for _, job := range jobs {
		m.logJob(job.JobID, "recover status=%s", job.Status)
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
	m.logJob(jobID, "created kind=%s image=%s plan_only=%t", prepared.request.PrepareKind, prepared.request.ImageID, prepared.request.PlanOnly)
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
	if err != nil {
		m.logJob(jobID, "lookup failed error=%v", err)
		return Status{}, false
	}
	if !ok {
		m.logJob(jobID, "lookup missing")
		return Status{}, false
	}
	tasks, err := m.queue.ListTasks(context.Background(), jobID)
	if err != nil {
		m.logJob(jobID, "task list failed error=%v", err)
		tasks = nil
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
		m.logJob(jobID, "delete blocked active_tasks=true")
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
		m.logJob(jobID, "delete force cancel")
		runner := m.getRunner(jobID)
		if runner != nil {
			runner.cancel()
			<-runner.done
		}
	}

	if err := m.queue.DeleteJob(context.Background(), jobID); err != nil {
		return deletion.DeleteResult{}, false
	}
	if err := m.removeJobDir(jobID); err != nil {
		m.logJob(jobID, "delete cleanup failed: %v", err)
		return deletion.DeleteResult{}, false
	}
	m.logJob(jobID, "deleted")
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
	jobSucceeded := false
	defer func() {
		if !jobSucceeded {
			m.cleanupRuntime(ctx, runner)
		}
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
	m.logJob(jobID, "running")

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
		if err := m.succeedPlan(jobID); err == nil {
			jobSucceeded = true
		}
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
		case "resolve_image":
			if strings.TrimSpace(task.ResolvedImageID) != "" && strings.TrimSpace(prepared.resolvedImageID) == "" {
				prepared.resolvedImageID = task.ResolvedImageID
			}
			if errResp := m.ensureResolvedImageID(ctx, jobID, &prepared, nil); errResp != nil {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusFailed, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), errResp)
				_ = m.failJob(jobID, errResp)
				return
			}
		case "state_execute":
			if err := m.executeStateTask(ctx, jobID, prepared, task); err != nil {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusFailed, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), err)
				_ = m.failJob(jobID, err)
				return
			}
			stateID = task.OutputStateID
		case "prepare_instance":
			result, errResp := m.createInstance(ctx, jobID, prepared, stateID)
			if errResp != nil {
				_ = m.updateTaskStatus(ctx, jobID, task.TaskID, StatusFailed, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), errResp)
				_ = m.failJob(jobID, errResp)
				return
			}
			if err := m.updateTaskStatus(ctx, jobID, task.TaskID, StatusSucceeded, nil, strPtr(m.now().UTC().Format(time.RFC3339Nano)), nil); err != nil {
				_ = m.failJob(jobID, errorResponse("internal_error", "cannot update task status", err.Error()))
				return
			}
			if err := m.succeed(jobID, *result); err == nil {
				jobSucceeded = true
			}
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
	result, errResp := m.createInstance(ctx, jobID, prepared, stateID)
	if errResp != nil {
		_ = m.failJob(jobID, errResp)
		return
	}
	if err := m.succeed(jobID, *result); err == nil {
		jobSucceeded = true
	}
}

func (m *Manager) loadOrPlanTasks(ctx context.Context, jobID string, prepared preparedRequest) ([]taskState, string, *ErrorResponse) {
	taskRecords, err := m.queue.ListTasks(ctx, jobID)
	if err != nil {
		return nil, "", errorResponse("internal_error", "cannot load tasks", err.Error())
	}
	if errResp := m.ensureResolvedImageID(ctx, jobID, &prepared, taskRecords); errResp != nil {
		return nil, "", errResp
	}
	if len(taskRecords) == 0 {
		if errResp := m.updateJobSignature(ctx, jobID, prepared); errResp != nil {
			return nil, "", errResp
		}
		tasks, stateID, errResp := m.buildPlan(prepared)
		if errResp != nil {
			return nil, "", errResp
		}
		m.logJob(jobID, "planned tasks count=%d state_id=%s", len(tasks), stateID)
		records := taskRecordsFromPlan(jobID, tasks)
		if err := m.queue.ReplaceTasks(ctx, jobID, records); err != nil {
			return nil, "", errorResponse("internal_error", "cannot store tasks", err.Error())
		}
		m.logJob(jobID, "stored tasks count=%d", len(tasks))
		m.trimCompletedJobs(ctx, prepared)
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

func (m *Manager) updateJobSignature(ctx context.Context, jobID string, prepared preparedRequest) *ErrorResponse {
	signature, errResp := m.computeJobSignature(prepared)
	if errResp != nil {
		return errResp
	}
	if err := m.queue.UpdateJob(ctx, jobID, queue.JobUpdate{Signature: &signature}); err != nil {
		return errorResponse("internal_error", "cannot update job signature", err.Error())
	}
	return nil
}

func (m *Manager) computeJobSignature(prepared preparedRequest) (string, *ErrorResponse) {
	taskHash, errResp := m.computeTaskHash(prepared)
	if errResp != nil {
		return "", errResp
	}
	imageID := prepared.effectiveImageID()
	if strings.TrimSpace(imageID) == "" {
		return "", errorResponse("internal_error", "resolved image id is required", "")
	}
	hasher := newStateHasher()
	hasher.write("task_hash", taskHash)
	hasher.write("image_id", imageID)
	hasher.write("plan_only", fmt.Sprintf("%t", prepared.request.PlanOnly))
	signature := hasher.sum()
	if signature == "" {
		return "", errorResponse("internal_error", "cannot compute job signature", "")
	}
	return signature, nil
}

func (m *Manager) trimCompletedJobs(ctx context.Context, prepared preparedRequest) {
	limit := maxIdenticalJobs(m.config)
	if limit <= 0 {
		return
	}
	signature, errResp := m.computeJobSignature(prepared)
	if errResp != nil {
		m.logJob("", "job retention skipped: %s", errResp.Message)
		return
	}
	jobs, err := m.queue.ListJobsBySignature(ctx, signature, []string{StatusSucceeded, StatusFailed})
	if err != nil {
		m.logJob("", "job retention failed: %v", err)
		return
	}
	if len(jobs) <= limit {
		return
	}
	for i := limit; i < len(jobs); i++ {
		jobID := jobs[i].JobID
		if err := m.queue.DeleteJob(ctx, jobID); err != nil {
			m.logJob(jobID, "job retention delete failed: %v", err)
			continue
		}
		if err := m.removeJobDir(jobID); err != nil {
			m.logJob(jobID, "job retention cleanup failed: %v", err)
		}
		m.logJob(jobID, "retention deleted")
	}
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
	resolvedImageID := ""
	if hasImageDigest(imageID) {
		resolvedImageID = imageID
	}
	return preparedRequest{
		request:         req,
		normalizedArgs:  prepared.normalizedArgs,
		argsNormalized:  prepared.argsNormalized,
		inputHashes:     prepared.inputHashes,
		filePaths:       prepared.filePaths,
		resolvedImageID: resolvedImageID,
	}, nil
}

func (m *Manager) buildPlan(prepared preparedRequest) ([]PlanTask, string, *ErrorResponse) {
	taskHash, errResp := m.computeTaskHash(prepared)
	if errResp != nil {
		return nil, "", errResp
	}
	imageID := prepared.effectiveImageID()
	if strings.TrimSpace(imageID) == "" {
		return nil, "", errorResponse("internal_error", "resolved image id is required", "")
	}
	stateID, errResp := m.computeOutputStateID("image", imageID, taskHash)
	if errResp != nil {
		return nil, "", errResp
	}
	cached, err := m.isStateCached(stateID)
	if err != nil {
		return nil, "", errorResponse("internal_error", "cannot check state cache", err.Error())
	}
	cachedFlag := cached

	tasks := make([]PlanTask, 0, 3)
	tasks = append(tasks, PlanTask{
		TaskID:      "plan",
		Type:        "plan",
		PlannerKind: prepared.request.PrepareKind,
	})
	if needsImageResolve(prepared.request.ImageID) {
		tasks = append(tasks, PlanTask{
			TaskID:          "resolve-image",
			Type:            "resolve_image",
			ImageID:         prepared.request.ImageID,
			ResolvedImageID: imageID,
		})
	}
	tasks = append(tasks,
		PlanTask{
			TaskID: "execute-0",
			Type:   "state_execute",
			Input: &TaskInput{
				Kind: "image",
				ID:   imageID,
			},
			TaskHash:      taskHash,
			OutputStateID: stateID,
			Cached:        &cachedFlag,
		},
		PlanTask{
			TaskID: "prepare-instance",
			Type:   "prepare_instance",
			Input: &TaskInput{
				Kind: "state",
				ID:   stateID,
			},
			InstanceMode: "ephemeral",
		},
	)
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
	m.logTask(jobID, taskID, "status=%s", status)
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
	m.logJob(jobID, "succeeded instance=%s state=%s", result.InstanceID, result.StateID)
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
	m.logJob(jobID, "succeeded plan_only=true")
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
	if errResp != nil {
		m.logJob(jobID, "failed code=%s message=%s", errResp.Code, errResp.Message)
	} else {
		m.logJob(jobID, "failed")
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

func (r *jobRunner) setRuntime(rt *jobRuntime) {
	r.mu.Lock()
	r.rt = rt
	r.mu.Unlock()
}

func (r *jobRunner) getRuntime() *jobRuntime {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rt
}

func (m *Manager) logJob(jobID string, format string, args ...any) {
	if strings.TrimSpace(jobID) == "" {
		log.Printf("prepare "+format, args...)
		return
	}
	args = append([]any{jobID}, args...)
	log.Printf("prepare job=%s "+format, args...)
}

func (m *Manager) logTask(jobID string, taskID string, format string, args ...any) {
	if strings.TrimSpace(jobID) == "" || strings.TrimSpace(taskID) == "" {
		log.Printf("prepare task "+format, args...)
		return
	}
	args = append([]any{jobID, taskID}, args...)
	log.Printf("prepare job=%s task=%s "+format, args...)
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
			JobID:           jobID,
			TaskID:          task.TaskID,
			Position:        i,
			Type:            task.Type,
			Status:          StatusQueued,
			PlannerKind:     nullableString(task.PlannerKind),
			InputKind:       nullableString(taskInputKind(task.Input)),
			InputID:         nullableString(taskInputID(task.Input)),
			ImageID:         nullableString(task.ImageID),
			ResolvedImageID: nullableString(task.ResolvedImageID),
			TaskHash:        nullableString(task.TaskHash),
			OutputStateID:   nullableString(task.OutputStateID),
			Cached:          task.Cached,
			InstanceMode:    nullableString(task.InstanceMode),
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
		TaskID:          task.TaskID,
		Type:            task.Type,
		PlannerKind:     valueOrEmpty(task.PlannerKind),
		Input:           input,
		ImageID:         valueOrEmpty(task.ImageID),
		ResolvedImageID: valueOrEmpty(task.ResolvedImageID),
		TaskHash:        valueOrEmpty(task.TaskHash),
		OutputStateID:   valueOrEmpty(task.OutputStateID),
		Cached:          task.Cached,
		InstanceMode:    valueOrEmpty(task.InstanceMode),
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
		TaskID:          task.TaskID,
		JobID:           task.JobID,
		Type:            task.Type,
		Status:          task.Status,
		PlannerKind:     valueOrEmpty(task.PlannerKind),
		Input:           input,
		ImageID:         valueOrEmpty(task.ImageID),
		ResolvedImageID: valueOrEmpty(task.ResolvedImageID),
		TaskHash:        valueOrEmpty(task.TaskHash),
		OutputStateID:   valueOrEmpty(task.OutputStateID),
		Cached:          task.Cached,
		InstanceMode:    valueOrEmpty(task.InstanceMode),
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

const defaultMaxIdenticalJobs = 2

func maxIdenticalJobs(cfg config.Store) int {
	if cfg == nil {
		return defaultMaxIdenticalJobs
	}
	value, err := cfg.Get("orchestrator.jobs.maxIdentical", true)
	if err != nil {
		return defaultMaxIdenticalJobs
	}
	if value == nil {
		return defaultMaxIdenticalJobs
	}
	if num, ok := configValueToInt(value); ok && num >= 0 {
		return num
	}
	return defaultMaxIdenticalJobs
}

func configValueToInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		if v != float32(int(v)) {
			return 0, false
		}
		return int(v), true
	case float64:
		if v != float64(int(v)) {
			return 0, false
		}
		return int(v), true
	case json.Number:
		if strings.ContainsAny(string(v), ".eE") {
			return 0, false
		}
		parsed, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func (m *Manager) removeJobDir(jobID string) error {
	if m == nil {
		return nil
	}
	if strings.TrimSpace(m.stateStoreRoot) == "" {
		return nil
	}
	if strings.TrimSpace(jobID) == "" {
		return nil
	}
	path := filepath.Join(m.stateStoreRoot, "jobs", jobID)
	return os.RemoveAll(path)
}

func strPtr(value string) *string {
	return &value
}

func (p preparedRequest) effectiveImageID() string {
	if strings.TrimSpace(p.resolvedImageID) != "" {
		return p.resolvedImageID
	}
	return p.request.ImageID
}

func hasImageDigest(imageID string) bool {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return false
	}
	at := strings.LastIndex(imageID, "@")
	return at != -1 && at+1 < len(imageID)
}

func needsImageResolve(imageID string) bool {
	return !hasImageDigest(imageID)
}

func (m *Manager) ensureResolvedImageID(ctx context.Context, jobID string, prepared *preparedRequest, tasks []queue.TaskRecord) *ErrorResponse {
	if prepared == nil {
		return errorResponse("internal_error", "prepared request is required", "")
	}
	if strings.TrimSpace(prepared.resolvedImageID) != "" {
		return nil
	}
	if resolved := resolvedImageFromTasks(tasks); resolved != "" {
		prepared.resolvedImageID = resolved
		return nil
	}
	if len(tasks) > 0 {
		prepared.resolvedImageID = prepared.request.ImageID
		return nil
	}
	if !needsImageResolve(prepared.request.ImageID) {
		prepared.resolvedImageID = prepared.request.ImageID
		return nil
	}
	resolved, err := m.runtime.ResolveImage(ctx, prepared.request.ImageID)
	if err != nil {
		return errorResponse("internal_error", "cannot resolve image", err.Error())
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return errorResponse("internal_error", "resolved image id is required", "")
	}
	prepared.resolvedImageID = resolved
	return nil
}

func resolvedImageFromTasks(tasks []queue.TaskRecord) string {
	for _, task := range tasks {
		if task.Type != "resolve_image" {
			continue
		}
		resolved := valueOrEmpty(task.ResolvedImageID)
		if strings.TrimSpace(resolved) != "" {
			return resolved
		}
	}
	return ""
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

func buildDSN(host string, port int) string {
	return fmt.Sprintf("postgres://sqlrs@%s:%d/postgres", host, port)
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
