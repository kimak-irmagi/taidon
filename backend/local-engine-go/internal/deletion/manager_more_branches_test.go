package deletion

import (
	"context"
	"errors"
	"testing"

	"sqlrs/engine/internal/store"
)

func TestDeleteInstanceReturnsNotFound(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, found, err := mgr.DeleteInstance(context.Background(), "missing", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if found {
		t.Fatalf("expected not found")
	}
}

func TestDeleteInstanceReturnsStopRuntimeError(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{
		InstanceID: "inst-1",
		StateID:    "state-1",
		RuntimeID:  strPtr("container-1"),
	}
	mgr, err := NewManager(Options{
		Store:   st,
		Runtime: &fakeRuntime{stopErr: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{Force: true}); err == nil {
		t.Fatalf("expected stop runtime error")
	}
}

func TestDeleteInstanceReturnsRemoveRuntimeDirError(t *testing.T) {
	st := newFakeStore()
	badPath := "bad\x00runtime-dir"
	st.instances["inst-1"] = store.InstanceEntry{
		InstanceID: "inst-1",
		StateID:    "state-1",
		RuntimeDir: &badPath,
	}
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{Force: true}); err == nil {
		t.Fatalf("expected remove runtime dir error")
	}
}

func TestDeleteStateReturnsGetStateError(t *testing.T) {
	st := newFakeStore()
	st.getStateErr = errors.New("boom")
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected get state error")
	}
}

func TestDeleteStateNonRecurseReturnsRemoveStateDirError(t *testing.T) {
	st := newFakeStore()
	st.states["state-1"] = store.StateEntry{StateID: "state-1", ImageID: "img"}
	mgr, err := NewManager(Options{
		Store:          st,
		StateStoreRoot: t.TempDir(),
		StateFS:        &fakeStateFS{stateDirErr: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected remove state dir error")
	}
}

func TestDeleteStateNonRecurseReturnsDeleteStateError(t *testing.T) {
	st := newFakeStore()
	st.states["state-1"] = store.StateEntry{StateID: "state-1"}
	st.deleteStateErr = errors.New("boom")
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected delete state error")
	}
}

func TestDeleteTreeInstanceReturnsStopRuntimeError(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{
		Store:   st,
		Runtime: &fakeRuntime{stopErr: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	err = mgr.deleteTree(context.Background(), DeleteNode{
		Kind:      "instance",
		ID:        "inst-1",
		RuntimeID: strPtr("container-1"),
	})
	if err == nil {
		t.Fatalf("expected stop runtime error")
	}
}

func TestDeleteTreeInstanceReturnsRemoveRuntimeDirError(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	badPath := "bad\x00runtime-dir"
	err = mgr.deleteTree(context.Background(), DeleteNode{
		Kind:       "instance",
		ID:         "inst-1",
		RuntimeDir: &badPath,
	})
	if err == nil {
		t.Fatalf("expected remove runtime dir error")
	}
}

func TestDeleteTreeStateReturnsRemoveStateDirError(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{
		Store:          st,
		StateStoreRoot: t.TempDir(),
		StateFS:        &fakeStateFS{stateDirErr: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	err = mgr.deleteTree(context.Background(), DeleteNode{
		Kind:    "state",
		ID:      "state-1",
		ImageID: strPtr("img"),
	})
	if err == nil {
		t.Fatalf("expected remove state dir error")
	}
}

func TestRemoveStateDirHandlesNilStateFSWithRoot(t *testing.T) {
	mgr := &Manager{stateStoreRoot: t.TempDir()}
	if err := mgr.removeStateDir(strPtr("img"), "state-1"); err != nil {
		t.Fatalf("removeStateDir: %v", err)
	}
}

func TestRemoveStateDirReturnsStateDirError(t *testing.T) {
	mgr := &Manager{
		stateStoreRoot: t.TempDir(),
		statefs:        &fakeStateFS{stateDirErr: errors.New("boom")},
	}
	if err := mgr.removeStateDir(strPtr("img"), "state-1"); err == nil {
		t.Fatalf("expected state dir error")
	}
}
