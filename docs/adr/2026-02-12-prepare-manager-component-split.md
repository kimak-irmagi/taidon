# ADR: prepare manager component split

Status: Proposed
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
