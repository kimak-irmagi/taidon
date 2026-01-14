package registry

import (
	"context"
	"strings"

	"sqlrs/engine/internal/id"
	"sqlrs/engine/internal/store"
)

type Registry struct {
	store store.Store
}

func New(store store.Store) *Registry {
	return &Registry{store: store}
}

func (r *Registry) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	return r.store.ListNames(ctx, filters)
}

func (r *Registry) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	return r.store.GetName(ctx, strings.TrimSpace(name))
}

func (r *Registry) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	return r.store.ListInstances(ctx, filters)
}

func (r *Registry) GetInstance(ctx context.Context, idOrName string) (store.InstanceEntry, bool, bool, error) {
	value := strings.TrimSpace(idOrName)
	if value == "" {
		return store.InstanceEntry{}, false, false, nil
	}
	if id.IsInstanceID(value) {
		entry, ok, err := r.store.GetInstance(ctx, value)
		if err != nil || ok {
			return entry, ok, false, err
		}
		entry, ok, err = r.getInstanceByName(ctx, value)
		return entry, ok, ok, err
	}
	entry, ok, err := r.getInstanceByName(ctx, value)
	return entry, ok, ok, err
}

func (r *Registry) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	return r.store.ListStates(ctx, filters)
}

func (r *Registry) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	return r.store.GetState(ctx, strings.TrimSpace(stateID))
}

func (r *Registry) Close() error {
	return r.store.Close()
}

func (r *Registry) getInstanceByName(ctx context.Context, name string) (store.InstanceEntry, bool, error) {
	entry, ok, err := r.store.GetName(ctx, name)
	if err != nil || !ok || entry.InstanceID == nil {
		return store.InstanceEntry{}, false, err
	}
	return r.store.GetInstance(ctx, *entry.InstanceID)
}
