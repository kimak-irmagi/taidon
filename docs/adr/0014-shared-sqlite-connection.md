# ADR 0014: shared sqlite connection for local engine state

Status: Accepted
Date: 2026-01-19

## Decision Record 1: shared sqlite connection for state.db

- Timestamp: 2026-01-19T20:22:44.8022220+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the local engine access state.db to avoid SQLITE_BUSY and self-contention?
- Alternatives:
  - Open separate `*sql.DB` handles for store and queue and rely on pooling.
  - Use one shared `*sql.DB` with a single connection and pass it to all modules.
  - Split store and queue into separate SQLite files.
- Decision: Use one shared `*sql.DB` with `MaxOpenConns=1` across modules.
- Rationale: Prevents cross-connection write locks and SQLITE_BUSY while keeping a single file and simple deployment.
