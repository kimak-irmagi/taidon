# Prepare Manager Refactor

Status: Implemented (2026-02-12)

## 1. Context

`internal/prepare` currently contains a very large `Manager` with mixed responsibilities:

- Job lifecycle coordination (`Submit`, `Recover`, `runJob`, events, retention).
- Task execution orchestration (runtime, psql/liquibase execution, instance creation).
- Snapshot orchestration (base init, dirty cache invalidation, snapshot lock/marker flow).

The current shape increases coupling and makes change-safety lower, especially around
`runJob`, `executeStateTask`, and snapshot/error handling.

## 2. Goals

- Keep external behavior and HTTP/API contracts unchanged.
- Reduce responsibility concentration in `prepare.Manager`.
- Isolate execution and snapshot logic from job coordination logic.
- Make refactoring incremental and test-safe.

## 3. Non-goals

- No endpoint changes.
- No storage schema changes.
- No queue model changes.
- No semantic changes to plan/task/event flow.

## 4. Target component structure

`prepare.Manager` remains an external facade used by `httpapi`, but delegates to three
internal components:

- `JobCoordinator`
  - Owns job lifecycle and queue/event transitions.
  - Owns planning/loading tasks and orchestrates step progression.
  - Calls `TaskExecutor` for `state_execute` and `prepare_instance`.
- `TaskExecutor`
  - Owns runtime acquisition/start/cleanup and step execution (`psql`, `lb`).
  - Owns instance creation from prepared state.
  - Delegates snapshot-specific decisions to `SnapshotOrchestrator`.
- `SnapshotOrchestrator`
  - Owns base-state initialization and dirty-state invalidation rules.
  - Owns snapshot pre/post checks and state cache hygiene helpers.

## 5. Dependency direction

- `Manager` -> `JobCoordinator`
- `JobCoordinator` -> `TaskExecutor`
- `TaskExecutor` -> `SnapshotOrchestrator`
- `SnapshotOrchestrator` -> storage/runtime/statefs/dbms abstractions via `Manager` dependencies

No new package-level dependencies are introduced. Components live in `internal/prepare`.

## 6. Public API impact

No change in public `prepare.Manager` surface:

- `Submit`, `Recover`, `Get`, `ListJobs`, `ListTasks`, `Delete`
- `EventsSince`, `WaitForEvent`

The HTTP handlers and CLI contracts stay intact.

## 7. Implementation status

The split is complete.

- `Manager` remains a facade for package/external callers.
- Heavy orchestration methods were moved from `Manager.*Impl` to:
  - `jobCoordinator` (`runJob`, planning, task loading, Liquibase planning flow).
  - `taskExecutor` (runtime/task execution, instance creation, Liquibase execution).
  - `snapshotOrchestrator` (base init + dirty cache invalidation).
- The previous bounce `Manager -> component -> Manager.*Impl` was removed.
- Queue/event transitions and error mapping remain behavior-compatible.

## 8. Risks and mitigations

- Risk: accidental behavior drift in task status/event sequencing.
  - Mitigation: keep facade methods, preserve queue/event write points, and run component + integration tests.
- Risk: concurrent runner/heartbeat regressions.
  - Mitigation: keep locking model and runner ownership unchanged in first pass.
- Risk: hidden coupling via direct `Manager` field access.
  - Mitigation: move incrementally, then optionally tighten internal contracts later.

## 9. Verification set

Implemented verification focuses on behavior contracts:

1. `prepare/job_coordinator_test.go`
   - `runJob` plan-only and execute flow transitions.
2. `prepare/task_executor_test.go`
   - runtime reuse and `state_execute` metadata/status updates.
3. `prepare/snapshot_orchestrator_test.go`
   - dirty cached state invalidation and base-state init guard.
4. `prepare/liquibase_consistency_test.go`
   - planning/execution consistency for Liquibase request preparation.
5. Existing `prepare` tests
   - preserve queue/event ordering, error mapping, and lifecycle invariants.
