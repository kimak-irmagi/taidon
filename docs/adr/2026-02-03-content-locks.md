# ADR 2026-02-03: Content locks for atomic fingerprints

Status: Accepted
Date: 2026-02-03

## Decision Record

- Timestamp: 2026-02-03T10:50:00+00:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How to prevent plan drift between content hashing and execution?
- Alternatives:
  - No locking; accept potential drift.
  - Lock inputs for the entire prepare job (plan + execute).
  - Lock inputs for the duration of each task that computes or applies content.
- Decision:
  - Acquire **read-locks** on all resolved inputs for the **duration of each task**
    (planning and each execution step).
  - Fail the task if any lock cannot be acquired.
- Rationale:
  - Prevents state fingerprints from being computed against content that changes
    before execution.
  - Avoids holding locks for the full job duration while still guaranteeing
    per-task atomicity.
