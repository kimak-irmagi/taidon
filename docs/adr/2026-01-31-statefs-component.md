# ADR: StateFS Component for Filesystem-Specific Snapshot Logic

- Conversation timestamp: 2026-01-31
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)

## Question

How should we isolate filesystem-specific snapshotting/cleanup logic so other engine components are not coupled to btrfs/overlay/copy details?

## Alternatives Considered

1. Keep `internal/snapshot` as-is and let other modules handle FS-specific cleanup (status quo).
2. Extend `internal/snapshot` with more helpers, but keep path layout and validation outside.
3. Introduce a new `StateFS` component that owns backend selection, validation, path layout, snapshot operations, and FS-aware cleanup; replace `internal/snapshot` usage in other modules.

## Decision

Adopt a `StateFS` component and fully replace direct snapshot manager usage in engine modules. `StateFS` owns validation, path layout, snapshot/clone, and FS-aware cleanup (e.g., btrfs subvolume deletion). Other components (prepare/run/deletion) depend only on the StateFS contract.

## Rationale

Filesystem-specific behaviors (mount validation, subvolume cleanup, path layout) materially affect correctness and are brittle when spread across modules. Centralizing these concerns in StateFS reduces coupling, simplifies other components, and allows backend-specific tactics to evolve independently.
