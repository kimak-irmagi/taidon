# ADR: Prepare job heartbeat and log events

- Timestamp: 2026-01-24
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)

## Question

How should the engine keep the CLI responsive during long-running prepare tasks,
and how should runtime/DBMS progress be surfaced?

## Alternatives considered

1) No heartbeat; rely solely on task status transitions.
2) Emit a new event type (e.g., `heartbeat`) on a fixed cadence.
3) Repeat the last task event with a new timestamp when the stream is quiet.

## Decision

- Emit log events for runtime/DBMS operations (Docker start, `psql` execution,
  snapshot prepare/resume), using `type: log` with a `message`.
- While a task is running and no new events are emitted, repeat the last task
  event with a new timestamp roughly every 500ms.

## Rationale

Repeating the last task event keeps the client UI alive without changing the
event schema or adding new types. Log events deliver more concrete progress
signals from Docker/Postgres, and the heartbeat only fills gaps when those
signals are absent.
