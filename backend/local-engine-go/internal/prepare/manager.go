package prepare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

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
	createdAt   time.Time
	startedAt   *time.Time
	finishedAt  *time.Time
	status      string
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

func (m *Manager) runJob(prepared preparedRequest, j *job) {
	started := m.now().UTC()
	j.setStatus(StatusRunning, started)

	stateID, argsNormalized, stateErr := m.computeState(prepared)
	if stateErr != nil {
		j.fail(m.now().UTC(), stateErr)
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
		PrepareArgsNormalized: argsNormalized,
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
		PrepareArgsNormalized: argsNormalized,
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

func (m *Manager) computeState(prepared preparedRequest) (string, string, *ErrorResponse) {
	hasher := newStateHasher()
	hasher.write("prepare_kind", prepared.request.PrepareKind)
	hasher.write("image_id", prepared.request.ImageID)
	for i, arg := range prepared.normalizedArgs {
		hasher.write(fmt.Sprintf("arg:%d", i), arg)
	}
	for i, input := range prepared.inputHashes {
		hasher.write(fmt.Sprintf("input:%d:%s", i, input.Kind), input.Value)
	}
	hasher.write("engine_version", m.version)
	stateID := hasher.sum()
	if stateID == "" {
		return "", "", errorResponse("internal_error", "cannot compute state id", "")
	}
	return stateID, prepared.argsNormalized, nil
}

func (j *job) snapshot() Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	status := Status{
		JobID:       j.id,
		Status:      j.status,
		PrepareKind: j.prepareKind,
		ImageID:     j.imageID,
		Result:      j.result,
		Error:       j.err,
	}
	status.CreatedAt = formatTime(j.createdAt)
	status.StartedAt = formatTimePtr(j.startedAt)
	status.FinishedAt = formatTimePtr(j.finishedAt)
	return status
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
