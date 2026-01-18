# ADR 0011: task executor and persistent queue

Status: Accepted
Date: 2026-01-19

## Decision Record 1: persist jobs/tasks/events in SQLite

- Timestamp: 2026-01-19T00:18:34+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine persist and recover job/task state?
- Alternatives:
  - Keep an in-memory queue and lose state on restart.
  - Store only jobs in SQLite and rebuild tasks from the plan on restart.
  - Persist jobs, tasks, and events in SQLite and recover from there.
- Decision: Persist jobs, tasks, and events in SQLite. On restart, reload queued/running work; when a task output state already exists, mark the task succeeded and continue, otherwise requeue it.
- Rationale: Survives restarts without losing visibility, keeps list endpoints accurate, and avoids replaying completed work.

## Decision Record 2: execution model, snapshots, and DBMS hooks

- Timestamp: 2026-01-19T00:18:34+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the executor run tasks and take snapshots?
- Alternatives:
  - Spin up one container per task and snapshot once at the end.
  - Stop the container for each snapshot and resume afterward.
  - Execute psql inside the container to avoid host tooling.
  - Use the Docker Engine SDK instead of the Docker CLI.
- Decision: Run one container per job and execute tasks sequentially. After each task, take a snapshot without stopping the container by using a DBMS connector (Postgres: fast shutdown with `pg_ctl`, then restart). Execute `psql` on the host and connect to the container. Use the Docker CLI in the MVP, behind a runtime interface for future SDK replacement. Default snapshot mode is "always" with a future job flag for none/always/final.
- Rationale: Minimizes container churn, keeps psql file handling simple, provides safe snapshots without full container stop, and keeps the runtime adapter minimal while preserving extensibility.

## Decision Record 3: task status events in job streams

- Timestamp: 2026-01-19T00:18:34+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should task status changes be surfaced to clients?
- Alternatives:
  - Emit only job-level status events.
  - Encode task identifiers in the message field only.
  - Introduce a separate task event stream endpoint.
- Decision: Emit task status changes as job events with an optional `task_id` field and stream them to all connected NDJSON clients.
- Rationale: Preserves a single stream per job while enabling structured task monitoring.

## Decision Record 4: force deletion cancels active jobs

- Timestamp: 2026-01-19T00:18:34+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What happens when `rm --force` targets an active job?
- Alternatives:
  - Remove the job record only and let execution finish.
  - Introduce a new `cancelled` terminal status.
  - Snapshot partial results before cancellation.
- Decision: `rm --force` cancels the running job via the executor. The runtime attempts a graceful stop and avoids snapshotting partial work; the job terminates as `failed` with a `cancelled` error.
- Rationale: Keeps safety semantics explicit and avoids partially valid snapshots.
