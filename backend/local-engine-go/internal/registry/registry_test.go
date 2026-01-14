package registry

import (
	"context"
	"testing"

	"sqlrs/engine/internal/store"
)

type fakeStore struct {
	names     map[string]store.NameEntry
	instances map[string]store.InstanceEntry
}

func (f *fakeStore) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	out := []store.NameEntry{}
	for _, entry := range f.names {
		if filters.InstanceID != "" {
			if entry.InstanceID == nil || *entry.InstanceID != filters.InstanceID {
				continue
			}
		}
		if filters.StateID != "" && entry.StateID != filters.StateID {
			continue
		}
		if filters.ImageID != "" && entry.ImageID != filters.ImageID {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func (f *fakeStore) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	entry, ok := f.names[name]
	return entry, ok, nil
}

func (f *fakeStore) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	out := []store.InstanceEntry{}
	for _, entry := range f.instances {
		if filters.StateID != "" && entry.StateID != filters.StateID {
			continue
		}
		if filters.ImageID != "" && entry.ImageID != filters.ImageID {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func (f *fakeStore) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	entry, ok := f.instances[instanceID]
	return entry, ok, nil
}

func (f *fakeStore) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	return nil, nil
}

func (f *fakeStore) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	return store.StateEntry{}, false, nil
}

func (f *fakeStore) Close() error {
	return nil
}

func TestGetInstanceResolution(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	aliasID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	aliasTarget := "cccccccccccccccccccccccccccccccc"
	fake := &fakeStore{
		names: map[string]store.NameEntry{
			"dev": {
				Name:       "dev",
				InstanceID: strPtr(instanceID),
				ImageID:    "image-1",
				StateID:    "state-1",
				Status:     store.NameStatusActive,
			},
			aliasID: {
				Name:       aliasID,
				InstanceID: strPtr(aliasTarget),
				ImageID:    "image-1",
				StateID:    "state-1",
				Status:     store.NameStatusActive,
			},
		},
		instances: map[string]store.InstanceEntry{
			instanceID: {InstanceID: instanceID},
			aliasTarget: {
				InstanceID: aliasTarget,
			},
		},
	}
	reg := New(fake)
	ctx := context.Background()

	entry, ok, resolvedByName, err := reg.GetInstance(ctx, instanceID)
	if err != nil || !ok || resolvedByName || entry.InstanceID != instanceID {
		t.Fatalf("expected id lookup, got entry=%+v ok=%v resolved=%v err=%v", entry, ok, resolvedByName, err)
	}

	entry, ok, resolvedByName, err = reg.GetInstance(ctx, aliasID)
	if err != nil || !ok || !resolvedByName || entry.InstanceID != aliasTarget {
		t.Fatalf("expected name fallback, got entry=%+v ok=%v resolved=%v err=%v", entry, ok, resolvedByName, err)
	}

	entry, ok, resolvedByName, err = reg.GetInstance(ctx, "dev")
	if err != nil || !ok || !resolvedByName || entry.InstanceID != instanceID {
		t.Fatalf("expected name lookup, got entry=%+v ok=%v resolved=%v err=%v", entry, ok, resolvedByName, err)
	}
}

func strPtr(value string) *string {
	return &value
}
