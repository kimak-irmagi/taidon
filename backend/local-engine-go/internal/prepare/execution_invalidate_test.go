package prepare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/engine/internal/statefs"
	"sqlrs/engine/internal/store"
)

type errorStore struct {
	fakeStore
	deleteErr error
}

func (e *errorStore) DeleteState(ctx context.Context, stateID string) error {
	if e.deleteErr != nil {
		return e.deleteErr
	}
	return e.fakeStore.DeleteState(ctx, stateID)
}

func TestInvalidateDirtyCachedStateEmptyID(t *testing.T) {
	mgr := newManagerWithStateFS(t, &fakeStore{}, &fakeStateFS{})
	dirty, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "")
	if dirty || errResp != nil {
		t.Fatalf("expected no-op, got dirty=%v err=%+v", dirty, errResp)
	}
}

func TestInvalidateDirtyCachedStateGetStateError(t *testing.T) {
	store := &fakeStore{getStateErr: errors.New("boom")}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})
	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{request: Request{ImageID: "image-1"}}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot load cached state") {
		t.Fatalf("expected load error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateMissingImageID(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1"},
	}}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})
	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "resolved image id is required") {
		t.Fatalf("expected image id error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateResolvePathsError(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithStateFS(t, store, &errorStateFS{stateErr: errors.New("boom")})
	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot resolve cached state paths") {
		t.Fatalf("expected resolve error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateDirtyPID(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "postmaster.pid"), []byte("1"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	dirty, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp != nil || !dirty {
		t.Fatalf("expected dirty invalidation, got dirty=%v err=%+v", dirty, errResp)
	}
	if len(store.deletedStates) != 1 || store.deletedStates[0] != "state-1" {
		t.Fatalf("expected state deletion, got %+v", store.deletedStates)
	}
}

func TestInvalidateDirtyCachedStateDirtyPIDDeleteError(t *testing.T) {
	store := &errorStore{
		fakeStore: fakeStore{statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1"},
		}},
		deleteErr: errors.New("boom"),
	}
	mgr := newManagerWithStoreAndStateFS(t, store, &fakeStateFS{})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "postmaster.pid"), []byte("1"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot delete dirty cached state") {
		t.Fatalf("expected delete error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateDirtyPIDRemoveError(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithStoreAndStateFS(t, store, &fakeStateFS{removeErr: errors.New("boom")})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.stateDir, "postmaster.pid"), []byte("1"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot remove dirty cached state dir") {
		t.Fatalf("expected remove error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateMissingPGVersion(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	dirty, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp != nil || !dirty {
		t.Fatalf("expected invalidation, got dirty=%v err=%+v", dirty, errResp)
	}
}

func TestInvalidateDirtyCachedStateMissingPGVersionRemoveError(t *testing.T) {
	store := &fakeStore{statesByID: map[string]store.StateEntry{
		"state-1": {StateID: "state-1", ImageID: "image-1"},
	}}
	mgr := newManagerWithStateFS(t, store, &fakeStateFS{removeErr: errors.New("boom")})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot remove cached state dir missing PG_VERSION") {
		t.Fatalf("expected remove error, got %+v", errResp)
	}
}

func TestInvalidateDirtyCachedStateMissingPGVersionDeleteError(t *testing.T) {
	store := &errorStore{
		fakeStore: fakeStore{statesByID: map[string]store.StateEntry{
			"state-1": {StateID: "state-1", ImageID: "image-1"},
		}},
		deleteErr: errors.New("boom"),
	}
	mgr := newManagerWithStoreAndStateFS(t, store, &fakeStateFS{})

	paths, err := resolveStatePaths(mgr.stateStoreRoot, "image-1", "state-1", mgr.statefs)
	if err != nil {
		t.Fatalf("resolveStatePaths: %v", err)
	}
	if err := os.MkdirAll(paths.stateDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, errResp := mgr.invalidateDirtyCachedState(context.Background(), "job-1", preparedRequest{}, "state-1")
	if errResp == nil || !strings.Contains(errResp.Message, "cannot delete cached state missing PG_VERSION") {
		t.Fatalf("expected delete error, got %+v", errResp)
	}
}


func newManagerWithStoreAndStateFS(t *testing.T, st store.Store, fs statefs.StateFS) *Manager {
	t.Helper()
	mgr, err := NewManager(Options{
		Store:          st,
		Queue:          newQueueStore(t),
		Runtime:        &fakeRuntime{},
		StateFS:        fs,
		DBMS:           &fakeDBMS{},
		StateStoreRoot: filepath.Join(t.TempDir(), "state-store"),
		Version:        "v1",
		Psql:           containerPsqlRunner{runtime: &fakeRuntime{}},
		Liquibase:      hostLiquibaseRunner{},
		Now:            func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}
