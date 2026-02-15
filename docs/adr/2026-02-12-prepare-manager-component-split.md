# ADR: prepare manager component split

Status: Accepted
Date: 2026-02-12

## Decision Record 1: split prepare manager by orchestration roles

- Timestamp: 2026-02-12T21:55:13.2403948+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should `internal/prepare.Manager` stay as a single orchestration
  component, or be split into smaller internal components?
- Alternatives:
  - Keep a single `Manager` and only do local function extraction.
  - Split into two parts (`JobCoordinator` + one combined executor/snapshot component).
  - Split into three parts (`JobCoordinator` + `TaskExecutor` + `SnapshotOrchestrator`)
    while preserving `Manager` as facade.
- Decision: Split by three internal roles: `JobCoordinator`, `TaskExecutor`, and
  `SnapshotOrchestrator`; keep `prepare.Manager` as the public facade used by `httpapi`.
- Rationale: This split isolates lifecycle orchestration, runtime/task execution,
  and snapshot/cache-specific logic without API/storage changes; it reduces
  coupling and keeps migration incremental.

## Decision Record 2: complete split by moving method bodies to components

- Timestamp: 2026-02-15T09:20:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should we keep the transitional `Manager -> component -> Manager.*Impl`
  indirection, or move heavy method bodies directly into component methods?
- Alternatives:
  - Keep transitional indirection (`Manager` wrappers + component proxies + `*Impl` on `Manager`).
  - Revert split and keep logic on `Manager`.
  - Complete split: keep `Manager` facade, move heavy logic to
    `jobCoordinator`/`taskExecutor`/`snapshotOrchestrator` methods.
- Decision: Complete split and remove `Manager.*Impl` transition layer while
  preserving `Manager` as facade.
- Rationale: Removes redundant boilerplate and hidden bounce calls, makes
  component ownership explicit in code, and keeps external behavior unchanged.

## Decision Record 3: rename facade type to PrepareService

- Timestamp: 2026-02-15T09:55:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should we keep facade type name `Manager` or rename it to
  `PrepareService` now that coordinator/executor/snapshot split is complete?
- Alternatives:
  - Keep `Manager` as the public facade type.
  - Rename to `PrepareService` and remove old symbol immediately.
  - Rename to `PrepareService` and keep `Manager` as a compatibility alias during migration.
- Decision: Rename the public facade type to `PrepareService` and keep
  backward compatibility via alias/constructor wrapper.
- Rationale: Improves API readability and role clarity while avoiding broad
  breakage in existing tests/integrations.

## Decision Record 4: remove temporary compatibility alias

- Timestamp: 2026-02-15T10:15:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should temporary compatibility symbols (`type Manager = PrepareService`,
  `NewManager`) remain in the codebase?
- Alternatives:
  - Keep temporary compatibility symbols for an additional transition window.
  - Remove compatibility symbols immediately and migrate all references.
- Decision: Remove compatibility symbols and keep only `PrepareService` and
  `NewPrepareService`.
- Rationale: Codebase is already migrated; keeping aliases would prolong
  naming ambiguity without practical benefit.
