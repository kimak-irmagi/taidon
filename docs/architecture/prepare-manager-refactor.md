# Prepare Manager Refactor (Proposed)

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

## 7. Migration plan

### Phase 1: Introduce components and delegation wrappers

- Add internal structs:
  - `jobCoordinator`
  - `taskExecutor`
  - `snapshotOrchestrator`
- Initialize them in `NewManager`.
- Keep existing `Manager` methods, but convert heavy methods to thin delegating wrappers.

### Phase 2: Move method bodies by responsibility

- Move job orchestration bodies to `jobCoordinator`.
- Move runtime/task execution bodies to `taskExecutor`.
- Move snapshot cache/base-init bodies to `snapshotOrchestrator`.

### Phase 3: Cleanups and duplication reduction

- Keep behavior identical but remove obvious duplication points in Liquibase execution setup.
- Ensure all moved code paths preserve existing queue/event writes and error mapping.

## 8. Risks and mitigations

- Risk: accidental behavior drift in task status/event sequencing.
  - Mitigation: keep delegation wrappers and existing tests green while moving bodies.
- Risk: concurrent runner/heartbeat regressions.
  - Mitigation: keep locking model and runner ownership unchanged in first pass.
- Risk: hidden coupling via direct `Manager` field access.
  - Mitigation: move incrementally, then optionally tighten internal contracts later.

## 9. Test design (for approval before implementation)

New tests to add:

1. `prepare/manager_delegation_test.go`
   - Verifies `Manager` delegates heavy flows to coordinator/executor/orchestrator.
2. `prepare/job_coordinator_test.go`
   - Covers `runJob` transitions for plan-only and execute flows (queued -> running -> succeeded/failed).
3. `prepare/task_executor_test.go`
   - Covers runtime acquisition/reuse and `state_execute` status update behavior.
4. `prepare/snapshot_orchestrator_test.go`
   - Covers dirty cached state invalidation and base-state init guard behavior.
5. `prepare/liquibase_consistency_test.go`
   - Guards parity between planning-time and execution-time Liquibase argument/environment preparation.

Existing tests should remain and continue validating behavior invariants.
