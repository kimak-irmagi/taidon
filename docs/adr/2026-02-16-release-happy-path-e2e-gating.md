# ADR: release happy-path E2E gating for MVP

Status: Accepted
Date: 2026-02-16

## Decision Record 1: MVP release gating scope by platform

Status: Obsolete (superseded by
[2026-02-19-release-e2e-btrfs-matrix.md](2026-02-19-release-e2e-btrfs-matrix.md))

- Timestamp: 2026-02-16T16:38:17.8397425+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should MVP release promotion be blocked by full happy-path E2E on all
  three platforms immediately, or use a phased rollout?
- Alternatives:
  - Require full blocking E2E on Linux, Windows, and macOS immediately.
  - Use phased gating: Linux full E2E blocking; Windows/macOS smoke checks only.
- Decision: Use phased gating for the current task: Linux full E2E is blocking,
  Windows/macOS run smoke checks; then move to full blocking on all three
  platforms as the immediate next step.
- Rationale: This keeps release progress for MVP while explicitly committing to
  near-term strict three-platform gating.

## Decision Record 2: initial release-blocking happy-path scenarios

- Timestamp: 2026-02-16T16:38:17.8397425+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Which happy-path scenarios should be release-blocking in MVP?
- Alternatives:
  - Use broad scenario set including flights and Liquibase from the start.
  - Start with a compact psql-only blocking set and expand later.
- Decision: Use `hp-psql-chinook` and `hp-psql-sakila` as the release-blocking
  baseline scenarios.
- Rationale: These are representative, stable, and aligned with existing
  `examples/` assets while keeping initial release gating fast and reliable.

## Decision Record 3: Liquibase happy-path gating order

- Timestamp: 2026-02-16T16:38:17.8397425+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should Liquibase happy-path be blocking in MVP now?
- Alternatives:
  - Make Liquibase blocking immediately.
  - Keep Liquibase non-blocking until three-platform full blocking is in place.
- Decision: Keep Liquibase non-blocking for the current task; add Liquibase
  into blocking scope right after full blocking is enabled on all three
  platforms.
- Rationale: Prioritizes platform-wide core release confidence first, then
  increases gating strictness for provider-specific flows.

