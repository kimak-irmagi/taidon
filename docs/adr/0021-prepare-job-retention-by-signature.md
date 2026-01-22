# ADR 0021: prepare job retention by signature

Status: Accepted
Date: 2026-01-21

## Decision Record 1: cap identical prepare jobs after planning

- Timestamp: 2026-01-21T17:10:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine prevent unbounded growth of prepare jobs/tasks/events?
- Alternatives:
  - Time-based TTL garbage collection (periodic or on startup).
  - Manual cleanup via a dedicated CLI command.
  - Per-signature retention: keep only the newest N identical jobs.
- Decision: Limit the number of completed prepare jobs per signature, deleting the oldest jobs after planning a new one.
- Rationale: Keeps recent history for a given workflow while preventing unbounded growth, without requiring explicit user cleanup.

## Decision Record 2: remove job directories on deletion

- Timestamp: 2026-01-21T17:22:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should job retention delete disk artifacts under the state store?
- Alternatives:
  - Delete only SQLite rows for jobs/tasks/events.
  - Delete `state-store/jobs/<job_id>/runtime` only.
  - Delete the entire `state-store/jobs/<job_id>` directory.
- Decision: Delete the entire `state-store/jobs/<job_id>` directory when a job is removed.
- Rationale: Avoids orphaned folders and keeps the state store tidy.
