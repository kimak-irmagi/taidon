# ADR 0018: base state init lock for concurrent prepares

Status: Accepted
Date: 2026-01-21

## Decision Record 1: serialize base state initialization

- Timestamp: 2026-01-21T09:10:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should concurrent prepare jobs avoid racing `initdb` on the same base directory?
- Alternatives:
  - Do nothing; rely on best-effort `PG_VERSION` checks.
  - Use a filesystem lock plus a success marker in the base directory.
  - Use an atomic marker only (create an `inprogress` file with O_EXCL) and recover from stale markers.
- Decision: Use a filesystem lock file plus a success marker (`base/.init.lock` and `base/.init.ok`). Only the lock holder runs `initdb`; others wait for the marker or lock release.
- Rationale: Prevents concurrent `initdb` corruption and makes the base initialization deterministic across parallel jobs, while keeping the mechanism local and simple.
