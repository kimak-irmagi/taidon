# StateFS Component Structure

This document defines the `statefs` component that encapsulates filesystem- and snapshot-specific logic inside the engine. The rest of the engine must not depend on concrete filesystem details.

## 1. Goals

- Isolate all **filesystem-specific** behavior (btrfs/overlay/copy) behind a single contract.
- Centralize **store validation** (mounts, fs type, subvolume sanity).
- Own **path layout** decisions (base/state/runtime/job runtime).
- Keep prepare/run/deletion unaware of the underlying filesystem tactics.

## 2. Responsibilities

- Detect/validate the state store root and mount status.
- Select the backend strategy (btrfs/overlay/copy) based on config and FS capabilities.
- Provide path layout helpers for base/states/runtime/job runtime.
- Create required directories/subvolumes in a backend-aware way.
- Clone and snapshot states.
- Remove paths safely (e.g., delete btrfs subvolumes before `rm -rf`).
- Expose capabilities (e.g., requires DB stop).

## 3. Contract (draft)

```go
// Package statefs

type Capabilities struct {
    RequiresDBStop        bool
    SupportsWritableClone bool
    SupportsSendReceive   bool
}

type CloneResult struct {
    MountDir string
    Cleanup  func() error
}

type StateFS interface {
    Kind() string
    Capabilities() Capabilities

    // Validation
    Validate(root string) error

    // Path layout
    BaseDir(root, imageID string) (string, error)
    StatesDir(root, imageID string) (string, error)
    StateDir(root, imageID, stateID string) (string, error)
    JobRuntimeDir(root, jobID string) (string, error)

    // Storage operations
    EnsureBaseDir(ctx context.Context, baseDir string) error
    EnsureStateDir(ctx context.Context, stateDir string) error
    Clone(ctx context.Context, srcDir, destDir string) (CloneResult, error)
    Snapshot(ctx context.Context, srcDir, destDir string) error
    RemovePath(ctx context.Context, path string) error
}
```

Notes:
- `RemovePath` encapsulates FS-specific deletion rules. For btrfs it deletes subvolumes, for copy it uses `os.RemoveAll`.
- `Validate` handles mount checks and fs-type validation (e.g., btrfs is mounted and visible to the engine).
- Path layout is owned here to allow per-backend layout changes without touching prepare/deletion.

## 4. Integration Points

- `prepare.PrepareService` facade and internal prepare components
  - `jobCoordinator` uses `StateFS.Validate` before execution.
  - `taskExecutor` uses `Clone` and `EnsureStateDir` for runtime/state transitions.
  - `snapshotOrchestrator` uses `EnsureBaseDir`, `Snapshot`, and dirty-state cleanup helpers.
  - prepare runtime paths are derived via `StateFS` layout helpers.
- `deletion.Manager`
  - uses `RemovePath` for runtime directories (instances)
- `run.Manager`
  - uses `StateFS` paths when recreating runtime containers

## 5. Package Placement

- New package: `internal/statefs`
- Existing `internal/snapshot` becomes internal to `statefs` or is replaced entirely.

## 6. Open Questions

- Should `StateFS` expose a single `PathLayout` object instead of individual path helpers?
- Do we need distinct removal methods (`RemoveRuntimeDir`, `RemoveStateDir`) or is `RemovePath` sufficient?

