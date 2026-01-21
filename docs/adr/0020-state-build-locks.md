# ADR 0020: state build lock and marker

Status: Accepted
Date: 2026-01-21

## Decision Record 1: serialize state snapshotting with filesystem locks

- Timestamp: 2026-01-21T11:15:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should concurrent prepare jobs coordinate when building the same state?
- Alternatives:
  - Use a database-level lock table to coordinate state builders.
  - Use per-state filesystem lock + marker files in the state directory.
- Decision: Use per-state filesystem locks and markers (`states/<state_id>/.build.lock`, `states/<state_id>/.build.ok`). Jobs wait for the marker or lock to clear before proceeding.
- Rationale: Simple to implement, works across multiple engine processes sharing the same state store, and avoids adding new DB tables.
