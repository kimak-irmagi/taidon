package registry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"sqlrs/engine/internal/store"
)

type fakeStore struct {
	listNames      func(context.Context, store.NameFilters) ([]store.NameEntry, error)
	getName        func(context.Context, string) (store.NameEntry, bool, error)
	listInstances  func(context.Context, store.InstanceFilters) ([]store.InstanceEntry, error)
	getInstance    func(context.Context, string) (store.InstanceEntry, bool, error)
	listStates     func(context.Context, store.StateFilters) ([]store.StateEntry, error)
	getState       func(context.Context, string) (store.StateEntry, bool, error)
	createState    func(context.Context, store.StateCreate) error
	createInstance func(context.Context, store.InstanceCreate) error
	deleteInstance func(context.Context, string) error
	deleteState    func(context.Context, string) error
	close          func() error
}

func (f *fakeStore) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	if f.listNames != nil {
		return f.listNames(ctx, filters)
	}
	return nil, nil
}

func (f *fakeStore) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	if f.getName != nil {
		return f.getName(ctx, name)
	}
	return store.NameEntry{}, false, nil
}

func (f *fakeStore) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	if f.listInstances != nil {
		return f.listInstances(ctx, filters)
	}
	return nil, nil
}

func (f *fakeStore) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	if f.getInstance != nil {
		return f.getInstance(ctx, instanceID)
	}
	return store.InstanceEntry{}, false, nil
}

func (f *fakeStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	if f.listStates != nil {
		return f.listStates(ctx, filters)
	}
	return nil, nil
}

func (f *fakeStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	if f.getState != nil {
		return f.getState(ctx, stateID)
	}
	return store.StateEntry{}, false, nil
}

func (f *fakeStore) CreateState(ctx context.Context, entry store.StateCreate) error {
	if f.createState != nil {
		return f.createState(ctx, entry)
	}
	return nil
}

func (f *fakeStore) CreateInstance(ctx context.Context, entry store.InstanceCreate) error {
	if f.createInstance != nil {
		return f.createInstance(ctx, entry)
	}
	return nil
}

func (f *fakeStore) DeleteInstance(ctx context.Context, instanceID string) error {
	if f.deleteInstance != nil {
		return f.deleteInstance(ctx, instanceID)
	}
	return nil
}

func (f *fakeStore) DeleteState(ctx context.Context, stateID string) error {
	if f.deleteState != nil {
		return f.deleteState(ctx, stateID)
	}
	return nil
}

func (f *fakeStore) Close() error {
	if f.close != nil {
		return f.close()
	}
	return nil
}

func TestRegistryListNamesPassesFilters(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	var got store.NameFilters
	fake.listNames = func(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
		got = filters
		return []store.NameEntry{{Name: "dev"}}, nil
	}
	reg := New(fake)
	_, err := reg.ListNames(ctx, store.NameFilters{InstanceID: "inst", StateID: "state", ImageID: "img"})
	if err != nil {
		t.Fatalf("ListNames: %v", err)
	}
	if got.InstanceID != "inst" || got.StateID != "state" || got.ImageID != "img" {
		t.Fatalf("unexpected filters: %+v", got)
	}
}

func TestRegistryListInstancesPassesFilters(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	var got store.InstanceFilters
	fake.listInstances = func(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
		got = filters
		return []store.InstanceEntry{{InstanceID: "inst"}}, nil
	}
	reg := New(fake)
	_, err := reg.ListInstances(ctx, store.InstanceFilters{StateID: "state", ImageID: "img", IDPrefix: "deadbeef"})
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if got.StateID != "state" || got.ImageID != "img" || got.IDPrefix != "deadbeef" {
		t.Fatalf("unexpected filters: %+v", got)
	}
}

func TestRegistryGetNameTrimsWhitespace(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	var got string
	fake.getName = func(ctx context.Context, name string) (store.NameEntry, bool, error) {
		got = name
		return store.NameEntry{Name: name}, true, nil
	}
	reg := New(fake)
	_, ok, err := reg.GetName(ctx, " dev ")
	if err != nil || !ok {
		t.Fatalf("GetName: %v ok=%v", err, ok)
	}
	if got != "dev" {
		t.Fatalf("expected trimmed name, got %q", got)
	}
}

func TestRegistryGetInstanceByID(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	calls := 0
	fake.getInstance = func(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
		calls++
		return store.InstanceEntry{InstanceID: instanceID}, true, nil
	}
	reg := New(fake)
	entry, ok, resolvedByName, err := reg.GetInstance(ctx, strings.Repeat("a", 32))
	if err != nil || !ok || resolvedByName {
		t.Fatalf("GetInstance: err=%v ok=%v byName=%v", err, ok, resolvedByName)
	}
	if entry.InstanceID == "" || calls != 1 {
		t.Fatalf("unexpected entry: %+v calls=%d", entry, calls)
	}
}

func TestRegistryGetInstanceFallsBackToName(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	calls := 0
	fake.getInstance = func(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
		calls++
		if calls == 1 {
			return store.InstanceEntry{}, false, nil
		}
		return store.InstanceEntry{InstanceID: instanceID}, true, nil
	}
	fake.getName = func(ctx context.Context, name string) (store.NameEntry, bool, error) {
		id := strings.Repeat("b", 32)
		return store.NameEntry{Name: name, InstanceID: &id}, true, nil
	}
	reg := New(fake)
	entry, ok, resolvedByName, err := reg.GetInstance(ctx, strings.Repeat("a", 32))
	if err != nil || !ok || !resolvedByName {
		t.Fatalf("GetInstance: err=%v ok=%v byName=%v", err, ok, resolvedByName)
	}
	if entry.InstanceID != strings.Repeat("b", 32) || calls != 2 {
		t.Fatalf("unexpected entry: %+v calls=%d", entry, calls)
	}
}

func TestRegistryGetInstanceBlank(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	reg := New(fake)
	_, ok, resolvedByName, err := reg.GetInstance(ctx, " ")
	if err != nil || ok || resolvedByName {
		t.Fatalf("expected no match, got err=%v ok=%v byName=%v", err, ok, resolvedByName)
	}
}

func TestRegistryGetInstanceNameMissingID(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	fake.getName = func(ctx context.Context, name string) (store.NameEntry, bool, error) {
		return store.NameEntry{Name: name}, true, nil
	}
	reg := New(fake)
	_, ok, resolvedByName, err := reg.GetInstance(ctx, "dev")
	if err != nil || ok || resolvedByName {
		t.Fatalf("expected no instance id, got err=%v ok=%v byName=%v", err, ok, resolvedByName)
	}
}

func TestRegistryGetInstanceNameError(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	fake.getName = func(ctx context.Context, name string) (store.NameEntry, bool, error) {
		return store.NameEntry{}, false, errors.New("boom")
	}
	reg := New(fake)
	_, _, _, err := reg.GetInstance(ctx, "dev")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegistryGetInstanceIDError(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	fake.getInstance = func(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
		return store.InstanceEntry{}, false, errors.New("boom")
	}
	reg := New(fake)
	_, _, _, err := reg.GetInstance(ctx, strings.Repeat("a", 32))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegistryListStatesAndGetState(t *testing.T) {
	ctx := context.Background()
	fake := &fakeStore{}
	var gotFilters store.StateFilters
	fake.listStates = func(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
		gotFilters = filters
		return []store.StateEntry{{StateID: "state"}}, nil
	}
	var gotStateID string
	fake.getState = func(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
		gotStateID = stateID
		return store.StateEntry{StateID: stateID}, true, nil
	}
	reg := New(fake)
	_, err := reg.ListStates(ctx, store.StateFilters{Kind: "psql", ImageID: "img", IDPrefix: "abcd"})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if gotFilters.Kind != "psql" || gotFilters.ImageID != "img" || gotFilters.IDPrefix != "abcd" {
		t.Fatalf("unexpected filters: %+v", gotFilters)
	}
	_, ok, err := reg.GetState(ctx, " state ")
	if err != nil || !ok {
		t.Fatalf("GetState: %v ok=%v", err, ok)
	}
	if gotStateID != "state" {
		t.Fatalf("expected trimmed state id, got %q", gotStateID)
	}
}

func TestRegistryCloseCallsStore(t *testing.T) {
	fake := &fakeStore{}
	closed := false
	fake.close = func() error {
		closed = true
		return nil
	}
	reg := New(fake)
	if err := reg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed {
		t.Fatalf("expected Close to call store Close")
	}
}
