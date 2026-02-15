package deletion

import (
	"context"
	"errors"
	"testing"

	"sqlrs/engine/internal/store"
)

func newDeletionManagerForBuildStateNodeTest(t *testing.T, st *fakeStore) *Manager {
	t.Helper()
	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

func TestBuildStateNodeGetStateError(t *testing.T) {
	st := newFakeStore()
	st.getStateErr = errors.New("boom")
	mgr := newDeletionManagerForBuildStateNodeTest(t, st)
	if _, _, err := mgr.buildStateNode(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected get state error")
	}
}

func TestBuildStateNodeStateNotFound(t *testing.T) {
	st := newFakeStore()
	mgr := newDeletionManagerForBuildStateNodeTest(t, st)
	if _, _, err := mgr.buildStateNode(context.Background(), "missing", DeleteOptions{}); err == nil {
		t.Fatalf("expected state-not-found error")
	}
}

func TestBuildStateNodeListInstancesError(t *testing.T) {
	st := newFakeStore()
	st.states["state-1"] = store.StateEntry{StateID: "state-1", ImageID: "image-1"}
	st.listInstancesErr = errors.New("boom")
	mgr := newDeletionManagerForBuildStateNodeTest(t, st)
	if _, _, err := mgr.buildStateNode(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected list instances error")
	}
}

func TestBuildStateNodeListChildStatesError(t *testing.T) {
	st := newFakeStore()
	st.states["state-1"] = store.StateEntry{StateID: "state-1", ImageID: "image-1"}
	st.listStatesErr = errors.New("boom")
	mgr := newDeletionManagerForBuildStateNodeTest(t, st)
	if _, _, err := mgr.buildStateNode(context.Background(), "state-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected list child states error")
	}
}
