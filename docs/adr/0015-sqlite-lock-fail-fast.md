# ADR 0015: sqlite lock handling (fail fast)

Status: Accepted
Date: 2026-01-19

## Decision Record 1: no busy timeout for state.db

- Timestamp: 2026-01-19T20:34:04.3529132+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine handle external sqlite locks on state.db?
- Alternatives:
  - Set a busy timeout and retry when the file is locked.
  - Fail fast and surface the lock error immediately.
  - Split state.db into multiple files to reduce contention.
- Decision: Fail fast; do not set a busy timeout.
- Rationale: External locks violate engine expectations and should be visible immediately instead of being masked by waits.
