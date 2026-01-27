# ADR: Snapshotter contract and consistency boundary

Status: Accepted
Date: 2026-01-27

## Decision Record

- Timestamp: 2026-01-27
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the snapshotter contract express consistency and backend requirements?
- Alternatives:
  - Add an explicit `Snapshot(mode)` API with `clean` vs `crash-consistent`.
  - Keep a minimal `Snapshot` API and push consistency decisions to orchestration.
  - Implicitly assume a clean snapshot for all backends.
- Decision: Keep the snapshotter API minimal and add `Capabilities().requires_db_stop` to declare backend requirements. Consistency mode is chosen at the orchestration layer (prepare/run policy), which stops DBMS when required.
- Rationale: This avoids coupling adapters to policy decisions, keeps snapshotter implementations simple, and still allows orchestration to enforce strict consistency when needed.
