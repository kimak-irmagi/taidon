package prepare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"sqlrs/engine/internal/deletion"
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
	Version string
	Now     func() time.Time
	IDGen   func() (string, error)
	Async   bool
}

type Manager struct {
	store   store.Store
	version string
	now     func() time.Time
	idGen   func() (string, error)
	async   bool

	mu   sync.RWMutex
	jobs map[string]*job
}

type job struct {
	mu          sync.Mutex
	id          string
	prepareKind string
	imageID     string
	planOnly    bool
	argsNorm    string
	createdAt   time.Time
	startedAt   *time.Time
	finishedAt  *time.Time
	status      string
	tasks       []PlanTask
	result      *Result
	err         *ErrorResponse
	events      []Event
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
		version: opts.Version,
		now:     now,
		idGen:   idGen,
		async:   opts.Async,
		jobs:    map[string]*job{},
	}, nil
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
	now := m.now().UTC()
	j := &job{
		id:          jobID,
		prepareKind: prepared.request.PrepareKind,
		imageID:     prepared.request.ImageID,
		planOnly:    prepared.request.PlanOnly,
		argsNorm:    prepared.argsNormalized,
		createdAt:   now,
		status:      StatusQueued,
	}
	j.events = append(j.events, Event{
		Type:   "status",
		Ts:     now.Format(time.RFC3339Nano),
		Status: StatusQueued,
	})

	m.mu.Lock()
	m.jobs[jobID] = j
	m.mu.Unlock()

	if m.async {
		go m.runJob(prepared, j)
	} else {
		m.runJob(prepared, j)
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
	j, ok := m.getJob(jobID)
	if !ok {
		return Status{}, false
	}
	return j.snapshot(), true
}

func (m *Manager) ListJobs(jobID string) []JobEntry {
	jobs := m.snapshotJobs(jobID)
	if len(jobs) == 0 {
		return []JobEntry{}
	}
	entries := make([]JobEntry, 0, len(jobs))
	for _, j := range jobs {
		entries = append(entries, j.entry())
	}
	return entries
}

func (m *Manager) ListTasks(jobID string) []TaskEntry {
	jobs := m.snapshotJobs(jobID)
	if len(jobs) == 0 {
		return []TaskEntry{}
	}
	entries := []TaskEntry{}
	for _, j := range jobs {
		entries = append(entries, j.taskEntries()...)
	}
	return entries
}

func (m *Manager) Delete(jobID string, opts deletion.DeleteOptions) (deletion.DeleteResult, bool) {
	j, ok := m.getJob(jobID)
	if !ok {
		return deletion.DeleteResult{}, false
	}
	status := j.statusValue()
	node := deletion.DeleteNode{
		Kind: "job",
		ID:   jobID,
	}
	blocked := false
	if status == StatusRunning && !opts.Force {
		node.Blocked = deletion.BlockActiveTasks
		blocked = true
	}
	outcome := deletion.OutcomeDeleted
	if blocked {
		outcome = deletion.OutcomeBlocked
	} else if opts.DryRun {
		outcome = deletion.OutcomeWouldDelete
	}
	result := deletion.DeleteResult{
		DryRun:  opts.DryRun,
		Outcome: outcome,
		Root:    node,
	}
	if blocked || opts.DryRun {
		return result, true
	}
	m.mu.Lock()
	delete(m.jobs, jobID)
	m.mu.Unlock()
	return result, true
}

func (m *Manager) EventsSince(jobID string, index int) ([]Event, bool, bool) {
	j, ok := m.getJob(jobID)
	if !ok {
		return nil, false, false
	}
	events, done := j.eventsSince(index)
	return events, true, done
}

func (m *Manager) getJob(jobID string) (*job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[jobID]
	return j, ok
}

func (m *Manager) snapshotJobs(jobID string) []*job {
	if strings.TrimSpace(jobID) != "" {
		j, ok := m.getJob(jobID)
		if !ok {
			return nil
		}
		return []*job{j}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

func (m *Manager) runJob(prepared preparedRequest, j *job) {
	started := m.now().UTC()
	j.setStatus(StatusRunning, started)

	tasks, stateID, stateErr := m.buildPlan(prepared)
	if stateErr != nil {
		j.fail(m.now().UTC(), stateErr)
		return
	}
	j.setTasks(tasks)

	if prepared.request.PlanOnly {
		j.succeedPlan(m.now().UTC())
		return
	}

	instanceID, err := randomHex(16)
	if err != nil {
		j.fail(m.now().UTC(), errorResponse("internal_error", "cannot generate instance id", err.Error()))
		return
	}

	created := started.Format(time.RFC3339Nano)
	if err := m.store.CreateState(context.Background(), store.StateCreate{
		StateID:               stateID,
		StateFingerprint:      stateID,
		ImageID:               prepared.request.ImageID,
		PrepareKind:           prepared.request.PrepareKind,
		PrepareArgsNormalized: prepared.argsNormalized,
		CreatedAt:             created,
	}); err != nil {
		j.fail(m.now().UTC(), errorResponse("internal_error", "cannot store state", err.Error()))
		return
	}

	if err := m.store.CreateInstance(context.Background(), store.InstanceCreate{
		InstanceID: instanceID,
		StateID:    stateID,
		ImageID:    prepared.request.ImageID,
		CreatedAt:  created,
	}); err != nil {
		j.fail(m.now().UTC(), errorResponse("internal_error", "cannot store instance", err.Error()))
		return
	}

	result := Result{
		DSN:                   buildDSN(instanceID),
		InstanceID:            instanceID,
		StateID:               stateID,
		ImageID:               prepared.request.ImageID,
		PrepareKind:           prepared.request.PrepareKind,
		PrepareArgsNormalized: prepared.argsNormalized,
	}
	j.succeed(m.now().UTC(), result)
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

func (j *job) snapshot() Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	status := Status{
		JobID:                 j.id,
		Status:                j.status,
		PrepareKind:           j.prepareKind,
		ImageID:               j.imageID,
		PlanOnly:              j.planOnly,
		PrepareArgsNormalized: j.argsNorm,
		Tasks:                 append([]PlanTask(nil), j.tasks...),
		Result:                j.result,
		Error:                 j.err,
	}
	status.CreatedAt = formatTime(j.createdAt)
	status.StartedAt = formatTimePtr(j.startedAt)
	status.FinishedAt = formatTimePtr(j.finishedAt)
	return status
}

func (j *job) entry() JobEntry {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JobEntry{
		JobID:       j.id,
		Status:      j.status,
		PrepareKind: j.prepareKind,
		ImageID:     j.imageID,
		PlanOnly:    j.planOnly,
		CreatedAt:   formatTime(j.createdAt),
		StartedAt:   formatTimePtr(j.startedAt),
		FinishedAt:  formatTimePtr(j.finishedAt),
	}
}

func (j *job) taskEntries() []TaskEntry {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.tasks) == 0 {
		return nil
	}
	entries := make([]TaskEntry, 0, len(j.tasks))
	for _, task := range j.tasks {
		entries = append(entries, TaskEntry{
			TaskID:        task.TaskID,
			JobID:         j.id,
			Type:          task.Type,
			Status:        j.status,
			PlannerKind:   task.PlannerKind,
			Input:         task.Input,
			TaskHash:      task.TaskHash,
			OutputStateID: task.OutputStateID,
			Cached:        task.Cached,
			InstanceMode:  task.InstanceMode,
		})
	}
	return entries
}

func (j *job) eventsSince(index int) ([]Event, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if index < 0 {
		index = 0
	}
	if index > len(j.events) {
		index = len(j.events)
	}
	events := append([]Event(nil), j.events[index:]...)
	done := j.status == StatusSucceeded || j.status == StatusFailed
	return events, done
}

func (j *job) statusValue() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

func (j *job) setStatus(status string, when time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if status == StatusRunning {
		j.startedAt = &when
	}
	j.status = status
	j.events = append(j.events, Event{
		Type:   "status",
		Ts:     when.Format(time.RFC3339Nano),
		Status: status,
	})
}

func (j *job) setTasks(tasks []PlanTask) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(tasks) == 0 {
		j.tasks = nil
		return
	}
	j.tasks = append([]PlanTask(nil), tasks...)
}

func (j *job) succeed(when time.Time, result Result) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = StatusSucceeded
	j.finishedAt = &when
	j.result = &result
	j.events = append(j.events,
		Event{
			Type:   "status",
			Ts:     when.Format(time.RFC3339Nano),
			Status: StatusSucceeded,
		},
		Event{
			Type:   "result",
			Ts:     when.Format(time.RFC3339Nano),
			Result: &result,
		},
	)
}

func (j *job) succeedPlan(when time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = StatusSucceeded
	j.finishedAt = &when
	j.events = append(j.events,
		Event{
			Type:   "status",
			Ts:     when.Format(time.RFC3339Nano),
			Status: StatusSucceeded,
		},
	)
}

func (j *job) fail(when time.Time, errResp *ErrorResponse) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status = StatusFailed
	j.finishedAt = &when
	j.err = errResp
	j.events = append(j.events,
		Event{
			Type:   "status",
			Ts:     when.Format(time.RFC3339Nano),
			Status: StatusFailed,
		},
		Event{
			Type:  "error",
			Ts:    when.Format(time.RFC3339Nano),
			Error: errResp,
		},
	)
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
