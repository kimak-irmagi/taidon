package prepare

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

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

func TestNewManagerRequiresStore(t *testing.T) {
	if _, err := NewManager(Options{}); err == nil {
		t.Fatalf("expected error when store is nil")
	}
}

func TestSubmitRejectsInvalidKind(t *testing.T) {
	mgr, err := NewManager(Options{Store: &fakeStore{}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = mgr.Submit(context.Background(), Request{PrepareKind: "", ImageID: "img"})
	expectValidationError(t, err, "prepare_kind is required")

	_, err = mgr.Submit(context.Background(), Request{PrepareKind: "liquibase", ImageID: "img"})
	expectValidationError(t, err, "unsupported prepare_kind")
}

func TestSubmitRejectsMissingImageID(t *testing.T) {
	mgr, err := NewManager(Options{Store: &fakeStore{}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = mgr.Submit(context.Background(), Request{PrepareKind: "psql"})
	expectValidationError(t, err, "image_id is required")
}

func TestSubmitStoresStateAndInstance(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mgr, err := NewManager(Options{
		Store:   store,
		Version: "v1",
		Now:     func() time.Time { return now },
		IDGen:   func() (string, error) { return "job-1", nil },
		Async:   false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	accepted, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if accepted.JobID != "job-1" || accepted.StatusURL == "" || accepted.EventsURL == "" {
		t.Fatalf("unexpected accepted: %+v", accepted)
	}
	if len(store.states) != 1 || len(store.instances) != 1 {
		t.Fatalf("expected state and instance to be stored")
	}
	if store.states[0].PrepareKind != "psql" || store.states[0].ImageID != "image-1" {
		t.Fatalf("unexpected state: %+v", store.states[0])
	}
	if store.instances[0].StateID == "" || store.instances[0].InstanceID == "" {
		t.Fatalf("unexpected instance: %+v", store.instances[0])
	}

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusSucceeded || status.Result == nil {
		t.Fatalf("unexpected status: %+v", status)
	}

	events, ok, done := mgr.EventsSince("job-1", -1)
	if !ok || !done || len(events) == 0 {
		t.Fatalf("unexpected events: ok=%v done=%v len=%d", ok, done, len(events))
	}

	events, ok, done = mgr.EventsSince("job-1", 100)
	if !ok || !done || len(events) != 0 {
		t.Fatalf("unexpected events slice: ok=%v done=%v len=%d", ok, done, len(events))
	}
}

func TestSubmitPlanOnlyBuildsTasks(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	mgr, err := NewManager(Options{
		Store:   store,
		Version: "v1",
		Now:     func() time.Time { return now },
		IDGen:   func() (string, error) { return "job-1", nil },
		Async:   false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	accepted, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
		PlanOnly:    true,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if accepted.JobID == "" {
		t.Fatalf("expected job id")
	}

	if len(store.states) != 0 || len(store.instances) != 0 {
		t.Fatalf("expected no state or instance creation")
	}

	status, ok := mgr.Get("job-1")
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
	if status.Tasks[1].Type != "state_execute" || status.Tasks[1].OutputStateID == "" {
		t.Fatalf("unexpected execute task: %+v", status.Tasks[1])
	}
	if status.Tasks[1].Cached == nil {
		t.Fatalf("expected cached flag")
	}
}

func TestSubmitPlanOnlyCachedFlag(t *testing.T) {
	fake := &fakeStore{statesByID: map[string]store.StateEntry{}}
	mgr, err := NewManager(Options{
		Store:   fake,
		Version: "v1",
		Now:     func() time.Time { return time.Now().UTC() },
		IDGen:   func() (string, error) { return "job-1", nil },
		Async:   false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

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

func TestSubmitCreateStateFails(t *testing.T) {
	store := &fakeStore{createStateErr: errors.New("boom")}
	mgr, err := NewManager(Options{
		Store: store,
		IDGen: func() (string, error) { return "job-1", nil },
		Async: false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusFailed || status.Error == nil || status.Error.Code != "internal_error" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSubmitCreateInstanceFails(t *testing.T) {
	store := &fakeStore{createInstanceErr: errors.New("boom")}
	mgr, err := NewManager(Options{
		Store: store,
		IDGen: func() (string, error) { return "job-1", nil },
		Async: false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	status, ok := mgr.Get("job-1")
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusFailed || status.Error == nil || status.Error.Code != "internal_error" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSubmitInstanceIDFails(t *testing.T) {
	store := &fakeStore{}
	mgr, err := NewManager(Options{
		Store: store,
		IDGen: func() (string, error) { return "job-1", nil },
		Async: false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

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
	if !ok {
		t.Fatalf("expected job to exist")
	}
	if status.Status != StatusFailed || status.Error == nil || status.Error.Message != "cannot generate instance id" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSubmitJobIDError(t *testing.T) {
	mgr, err := NewManager(Options{
		Store: &fakeStore{},
		IDGen: func() (string, error) { return "", errors.New("id error") },
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := mgr.Submit(context.Background(), Request{
		PrepareKind: "psql",
		ImageID:     "image-1",
		PsqlArgs:    []string{"-c", "select 1"},
	}); err == nil {
		t.Fatalf("expected id error")
	}
}

func TestEventsSinceUnknown(t *testing.T) {
	mgr, err := NewManager(Options{Store: &fakeStore{}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	events, ok, done := mgr.EventsSince("missing", 0)
	if ok || done || events != nil {
		t.Fatalf("expected missing job, got ok=%v done=%v events=%v", ok, done, events)
	}
}

func TestGetUnknownJob(t *testing.T) {
	mgr, err := NewManager(Options{Store: &fakeStore{}})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, ok := mgr.Get("missing"); ok {
		t.Fatalf("expected missing job")
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
