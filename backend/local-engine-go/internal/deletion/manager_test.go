package deletion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
)

type fakeConn struct {
	counts map[string]int
	err    error
}

func (f fakeConn) ActiveConnections(ctx context.Context, instanceID string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.counts == nil {
		return 0, nil
	}
	return f.counts[instanceID], nil
}

type fakeStore struct {
	instances         map[string]store.InstanceEntry
	states            map[string]store.StateEntry
	deletedInstances  []string
	deletedStates     []string
	listInstancesErr  error
	listStatesErr     error
	getInstanceErr    error
	getStateErr       error
	deleteInstanceErr error
	deleteStateErr    error
}

type fakeRuntime struct {
	stopCalls []string
	stopErr   error
}

func (f *fakeRuntime) InitBase(ctx context.Context, imageID string, dataDir string) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	return nil
}

func (f *fakeRuntime) ResolveImage(ctx context.Context, imageID string) (string, error) {
	if f.stopErr != nil {
		return "", f.stopErr
	}
	return imageID, nil
}

func (f *fakeRuntime) Start(ctx context.Context, req runtime.StartRequest) (runtime.Instance, error) {
	if f.stopErr != nil {
		return runtime.Instance{}, f.stopErr
	}
	return runtime.Instance{}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context, id string) error {
	f.stopCalls = append(f.stopCalls, id)
	if f.stopErr != nil {
		return f.stopErr
	}
	return nil
}

func (f *fakeRuntime) Exec(ctx context.Context, id string, req runtime.ExecRequest) (string, error) {
	if f.stopErr != nil {
		return "", f.stopErr
	}
	return "", nil
}

func (f *fakeRuntime) WaitForReady(ctx context.Context, id string, timeout time.Duration) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	return nil
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		instances: map[string]store.InstanceEntry{},
		states:    map[string]store.StateEntry{},
	}
}

func (f *fakeStore) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	return nil, nil
}

func (f *fakeStore) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	return store.NameEntry{}, false, nil
}

func (f *fakeStore) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	if f.listInstancesErr != nil {
		return nil, f.listInstancesErr
	}
	var out []store.InstanceEntry
	for _, entry := range f.instances {
		if filters.StateID != "" && entry.StateID != filters.StateID {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	return out, nil
}

func (f *fakeStore) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	if f.getInstanceErr != nil {
		return store.InstanceEntry{}, false, f.getInstanceErr
	}
	entry, ok := f.instances[instanceID]
	return entry, ok, nil
}

func (f *fakeStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	if f.listStatesErr != nil {
		return nil, f.listStatesErr
	}
	var out []store.StateEntry
	for _, entry := range f.states {
		if filters.ParentID != "" {
			if entry.ParentStateID == nil || *entry.ParentStateID != filters.ParentID {
				continue
			}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StateID < out[j].StateID })
	return out, nil
}

func (f *fakeStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	if f.getStateErr != nil {
		return store.StateEntry{}, false, f.getStateErr
	}
	entry, ok := f.states[stateID]
	return entry, ok, nil
}

func (f *fakeStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	return nil
}

func (f *fakeStore) CreateInstance(ctx context.Context, entry store.InstanceCreate) error {
	return nil
}

func (f *fakeStore) DeleteInstance(ctx context.Context, instanceID string) error {
	if f.deleteInstanceErr != nil {
		return f.deleteInstanceErr
	}
	delete(f.instances, instanceID)
	f.deletedInstances = append(f.deletedInstances, instanceID)
	return nil
}

func (f *fakeStore) DeleteState(ctx context.Context, stateID string) error {
	if f.deleteStateErr != nil {
		return f.deleteStateErr
	}
	delete(f.states, stateID)
	f.deletedStates = append(f.deletedStates, stateID)
	return nil
}

func (f *fakeStore) Close() error {
	return nil
}

func TestNewManagerRequiresStore(t *testing.T) {
	_, err := NewManager(Options{})
	if err == nil {
		t.Fatalf("expected error when store is nil")
	}
	if err.Error() == "" {
		t.Fatalf("expected error message")
	}
}

func TestDeleteInstanceBlockedByConnections(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "state-1"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{counts: map[string]int{"inst-1": 2}},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !found {
		t.Fatalf("expected instance to be found")
	}
	if result.Outcome != OutcomeBlocked || result.Root.Blocked != BlockActiveConnections {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.instances["inst-1"]; !ok {
		t.Fatalf("expected instance to remain")
	}
}

func TestDeleteInstanceDryRun(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "state-1"}

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !found {
		t.Fatalf("expected instance to be found")
	}
	if result.Outcome != OutcomeWouldDelete || !result.DryRun {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.instances["inst-1"]; !ok {
		t.Fatalf("expected instance to remain on dry run")
	}
}

func TestDeleteInstanceForceDeletes(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "state-1"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{counts: map[string]int{"inst-1": 1}},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{Force: true})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !found {
		t.Fatalf("expected instance to be found")
	}
	if result.Outcome != OutcomeDeleted || result.Root.Blocked != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.instances["inst-1"]; ok {
		t.Fatalf("expected instance to be deleted")
	}
}

func TestDeleteInstanceStopsRuntime(t *testing.T) {
	st := newFakeStore()
	dir := filepath.Join(t.TempDir(), "runtime-dir")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	st.instances["inst-1"] = store.InstanceEntry{
		InstanceID: "inst-1",
		StateID:    "state-1",
		RuntimeID:  strPtr("container-1"),
		RuntimeDir: strPtr(dir),
	}

	fake := &fakeRuntime{}
	mgr, err := NewManager(Options{
		Store:   st,
		Runtime: fake,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, found, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{Force: true})
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !found {
		t.Fatalf("expected instance to be found")
	}
	if len(fake.stopCalls) != 1 || fake.stopCalls[0] != "container-1" {
		t.Fatalf("expected runtime stop for container-1, got %+v", fake.stopCalls)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected runtime dir to be removed")
	}
	if _, ok := st.instances["inst-1"]; ok {
		t.Fatalf("expected instance to be deleted")
	}
}

func TestDeleteStateNonRecurseBlocked(t *testing.T) {
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "root"}

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeBlocked || result.Root.Blocked != BlockHasDescendants {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.states["root"]; !ok {
		t.Fatalf("expected state to remain")
	}
}

func TestDeleteStateNonRecurseDeletes(t *testing.T) {
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeDeleted {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.states["root"]; ok {
		t.Fatalf("expected state to be deleted")
	}
}

func TestDeleteStateNonRecurseDryRun(t *testing.T) {
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeWouldDelete || !result.DryRun {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := st.states["root"]; !ok {
		t.Fatalf("expected state to remain on dry run")
	}
}

func TestDeleteStateRecurseBlockedByConnections(t *testing.T) {
	parent := "root"
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.states["child"] = store.StateEntry{StateID: "child", ParentStateID: &parent}
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "child"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{counts: map[string]int{"inst-1": 1}},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{Recurse: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeBlocked || result.Root.Blocked != BlockBlockedDescendant {
		t.Fatalf("unexpected result: %+v", result)
	}
	node := findNode(result.Root, "instance", "inst-1")
	if node == nil || node.Blocked != BlockActiveConnections {
		t.Fatalf("expected blocked child instance, got %+v", node)
	}
	if _, ok := st.states["root"]; !ok {
		t.Fatalf("expected state to remain")
	}
}

func TestDeleteStateRecurseForceDeletes(t *testing.T) {
	parent := "root"
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.states["child"] = store.StateEntry{StateID: "child", ParentStateID: &parent}
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "child"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{counts: map[string]int{"inst-1": 2}},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{Recurse: true, Force: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeDeleted {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(st.states) != 0 || len(st.instances) != 0 {
		t.Fatalf("expected all resources deleted")
	}
}

func TestDeleteStateRecurseStopsRuntime(t *testing.T) {
	parent := "root"
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.states["child"] = store.StateEntry{StateID: "child", ParentStateID: &parent}
	dir := filepath.Join(t.TempDir(), "runtime-dir")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	st.instances["inst-1"] = store.InstanceEntry{
		InstanceID: "inst-1",
		StateID:    "child",
		RuntimeID:  strPtr("container-1"),
		RuntimeDir: strPtr(dir),
	}

	fake := &fakeRuntime{}
	mgr, err := NewManager(Options{
		Store:   st,
		Runtime: fake,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result, found, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{Recurse: true, Force: true})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if !found {
		t.Fatalf("expected state to be found")
	}
	if result.Outcome != OutcomeDeleted {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(fake.stopCalls) != 1 || fake.stopCalls[0] != "container-1" {
		t.Fatalf("expected runtime stop for container-1, got %+v", fake.stopCalls)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected runtime dir to be removed")
	}
	if len(st.instances) != 0 {
		t.Fatalf("expected all instances deleted")
	}
	if len(st.states) != 0 {
		t.Fatalf("expected all states deleted")
	}
}

func TestDeleteStateNotFound(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, found, err := mgr.DeleteState(context.Background(), "missing", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if found {
		t.Fatalf("expected not found")
	}
}

func TestDeleteInstancePropagatesErrors(t *testing.T) {
	st := newFakeStore()
	st.getInstanceErr = errors.New("boom")
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteInstance(context.Background(), "inst", DeleteOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteInstanceConnectionError(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "state-1"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{err: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteInstanceDeleteError(t *testing.T) {
	st := newFakeStore()
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "state-1"}
	st.deleteInstanceErr = errors.New("boom")

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteInstance(context.Background(), "inst-1", DeleteOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteStateListInstancesError(t *testing.T) {
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.listInstancesErr = errors.New("boom")

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteStateListStatesError(t *testing.T) {
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.listStatesErr = errors.New("boom")

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteStateRecurseConnectionError(t *testing.T) {
	parent := "root"
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.states["child"] = store.StateEntry{StateID: "child", ParentStateID: &parent}
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "child"}

	mgr, err := NewManager(Options{
		Store: st,
		Conn:  fakeConn{err: errors.New("boom")},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{Recurse: true}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteStateRecurseDeleteError(t *testing.T) {
	parent := "root"
	st := newFakeStore()
	st.states["root"] = store.StateEntry{StateID: "root"}
	st.states["child"] = store.StateEntry{StateID: "child", ParentStateID: &parent}
	st.instances["inst-1"] = store.InstanceEntry{InstanceID: "inst-1", StateID: "child"}
	st.deleteInstanceErr = errors.New("boom")

	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, _, err := mgr.DeleteState(context.Background(), "root", DeleteOptions{Recurse: true, Force: true}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDeleteTreeUnknownKind(t *testing.T) {
	st := newFakeStore()
	mgr, err := NewManager(Options{Store: st})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.deleteTree(context.Background(), DeleteNode{Kind: "unknown", ID: "x"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func findNode(root DeleteNode, kind, id string) *DeleteNode {
	if root.Kind == kind && root.ID == id {
		return &root
	}
	for _, child := range root.Children {
		if found := findNode(child, kind, id); found != nil {
			return found
		}
	}
	return nil
}

func strPtr(value string) *string {
	return &value
}
