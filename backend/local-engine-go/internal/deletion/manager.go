package deletion

import (
	"context"
	"os"
	"strings"

	"sqlrs/engine/internal/conntrack"
	"sqlrs/engine/internal/runtime"
	"sqlrs/engine/internal/store"
)

const (
	OutcomeDeleted     = "deleted"
	OutcomeWouldDelete = "would_delete"
	OutcomeBlocked     = "blocked"

	BlockActiveConnections = "active_connections"
	BlockActiveTasks       = "active_tasks"
	BlockHasDescendants    = "has_descendants"
	BlockBlockedDescendant = "blocked_by_descendant"
)

type Options struct {
	Store   store.Store
	Conn    conntrack.Tracker
	Runtime runtime.Runtime
}

type Manager struct {
	store   store.Store
	conn    conntrack.Tracker
	runtime runtime.Runtime
}

type DeleteOptions struct {
	Recurse bool
	Force   bool
	DryRun  bool
}

type DeleteResult struct {
	DryRun  bool       `json:"dry_run"`
	Outcome string     `json:"outcome"`
	Root    DeleteNode `json:"root"`
}

type DeleteNode struct {
	Kind        string       `json:"kind"`
	ID          string       `json:"id"`
	Connections *int         `json:"connections,omitempty"`
	Blocked     string       `json:"blocked,omitempty"`
	RuntimeID   *string      `json:"runtime_id,omitempty"`
	RuntimeDir  *string      `json:"-"`
	Children    []DeleteNode `json:"children,omitempty"`
}

func NewManager(opts Options) (*Manager, error) {
	if opts.Store == nil {
		return nil, storeError("store is required")
	}
	tracker := opts.Conn
	if tracker == nil {
		tracker = conntrack.Noop{}
	}
	return &Manager{store: opts.Store, conn: tracker, runtime: opts.Runtime}, nil
}

func (m *Manager) DeleteInstance(ctx context.Context, instanceID string, opts DeleteOptions) (DeleteResult, bool, error) {
	entry, ok, err := m.store.GetInstance(ctx, instanceID)
	if err != nil {
		return DeleteResult{}, false, err
	}
	if !ok {
		return DeleteResult{}, false, nil
	}

	connections, err := m.conn.ActiveConnections(ctx, instanceID)
	if err != nil {
		return DeleteResult{}, true, err
	}
	node := DeleteNode{
		Kind:        "instance",
		ID:          instanceID,
		Connections: &connections,
		RuntimeID:   entry.RuntimeID,
		RuntimeDir:  entry.RuntimeDir,
	}
	blocked := false
	if connections > 0 && !opts.Force {
		node.Blocked = BlockActiveConnections
		blocked = true
	}

	result := DeleteResult{
		DryRun:  opts.DryRun,
		Outcome: outcomeFor(blocked, opts.DryRun),
		Root:    node,
	}
	if blocked || opts.DryRun {
		return result, true, nil
	}
	if err := m.stopRuntime(ctx, entry.RuntimeID); err != nil {
		return DeleteResult{}, true, err
	}
	if err := removeRuntimeDir(entry.RuntimeDir); err != nil {
		return DeleteResult{}, true, err
	}
	if err := m.store.DeleteInstance(ctx, instanceID); err != nil {
		return DeleteResult{}, true, err
	}
	return result, true, nil
}

func (m *Manager) DeleteState(ctx context.Context, stateID string, opts DeleteOptions) (DeleteResult, bool, error) {
	_, ok, err := m.store.GetState(ctx, stateID)
	if err != nil {
		return DeleteResult{}, false, err
	}
	if !ok {
		return DeleteResult{}, false, nil
	}

	if !opts.Recurse {
		hasDescendants, err := m.hasDescendants(ctx, stateID)
		if err != nil {
			return DeleteResult{}, true, err
		}
		node := DeleteNode{
			Kind: "state",
			ID:   stateID,
		}
		if hasDescendants {
			node.Blocked = BlockHasDescendants
			return DeleteResult{
				DryRun:  opts.DryRun,
				Outcome: OutcomeBlocked,
				Root:    node,
			}, true, nil
		}

		result := DeleteResult{
			DryRun:  opts.DryRun,
			Outcome: outcomeFor(false, opts.DryRun),
			Root:    node,
		}
		if opts.DryRun {
			return result, true, nil
		}
		if err := m.store.DeleteState(ctx, stateID); err != nil {
			return DeleteResult{}, true, err
		}
		return result, true, nil
	}

	tree, blocked, err := m.buildStateNode(ctx, stateID, opts)
	if err != nil {
		return DeleteResult{}, true, err
	}
	result := DeleteResult{
		DryRun:  opts.DryRun,
		Outcome: outcomeFor(blocked, opts.DryRun),
		Root:    tree,
	}
	if blocked || opts.DryRun {
		return result, true, nil
	}
	if err := m.deleteTree(ctx, tree); err != nil {
		return DeleteResult{}, true, err
	}
	return result, true, nil
}

func (m *Manager) hasDescendants(ctx context.Context, stateID string) (bool, error) {
	instances, err := m.store.ListInstances(ctx, store.InstanceFilters{StateID: stateID})
	if err != nil {
		return false, err
	}
	if len(instances) > 0 {
		return true, nil
	}
	children, err := m.store.ListStates(ctx, store.StateFilters{ParentID: stateID})
	if err != nil {
		return false, err
	}
	return len(children) > 0, nil
}

func (m *Manager) buildStateNode(ctx context.Context, stateID string, opts DeleteOptions) (DeleteNode, bool, error) {
	node := DeleteNode{
		Kind: "state",
		ID:   stateID,
	}

	blocked := false
	instances, err := m.store.ListInstances(ctx, store.InstanceFilters{StateID: stateID})
	if err != nil {
		return DeleteNode{}, false, err
	}
	for _, entry := range instances {
		connections, err := m.conn.ActiveConnections(ctx, entry.InstanceID)
		if err != nil {
			return DeleteNode{}, false, err
		}
		child := DeleteNode{
			Kind:        "instance",
			ID:          entry.InstanceID,
			Connections: &connections,
			RuntimeID:   entry.RuntimeID,
			RuntimeDir:  entry.RuntimeDir,
		}
		if connections > 0 && !opts.Force {
			child.Blocked = BlockActiveConnections
			blocked = true
		}
		node.Children = append(node.Children, child)
	}

	children, err := m.store.ListStates(ctx, store.StateFilters{ParentID: stateID})
	if err != nil {
		return DeleteNode{}, false, err
	}
	for _, entry := range children {
		childNode, childBlocked, err := m.buildStateNode(ctx, entry.StateID, opts)
		if err != nil {
			return DeleteNode{}, false, err
		}
		if childBlocked {
			blocked = true
		}
		node.Children = append(node.Children, childNode)
	}

	if blocked {
		node.Blocked = BlockBlockedDescendant
	}
	return node, blocked, nil
}

func (m *Manager) deleteTree(ctx context.Context, node DeleteNode) error {
	for _, child := range node.Children {
		if err := m.deleteTree(ctx, child); err != nil {
			return err
		}
	}
	switch node.Kind {
	case "instance":
		if err := m.stopRuntime(ctx, node.RuntimeID); err != nil {
			return err
		}
		if err := removeRuntimeDir(node.RuntimeDir); err != nil {
			return err
		}
		return m.store.DeleteInstance(ctx, node.ID)
	case "state":
		return m.store.DeleteState(ctx, node.ID)
	default:
		return nil
	}
}

func outcomeFor(blocked bool, dryRun bool) string {
	if blocked {
		return OutcomeBlocked
	}
	if dryRun {
		return OutcomeWouldDelete
	}
	return OutcomeDeleted
}

func (m *Manager) stopRuntime(ctx context.Context, runtimeID *string) error {
	if m.runtime == nil || runtimeID == nil {
		return nil
	}
	id := strings.TrimSpace(*runtimeID)
	if id == "" {
		return nil
	}
	return m.runtime.Stop(ctx, id)
}

func removeRuntimeDir(runtimeDir *string) error {
	if runtimeDir == nil {
		return nil
	}
	dir := strings.TrimSpace(*runtimeDir)
	if dir == "" {
		return nil
	}
	return os.RemoveAll(dir)
}

type storeError string

func (e storeError) Error() string {
	return string(e)
}
