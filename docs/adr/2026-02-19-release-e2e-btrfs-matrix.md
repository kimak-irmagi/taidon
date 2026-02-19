# ADR: release E2E btrfs matrix expansion

Status: Accepted
Date: 2026-02-19

## Decision Record 1: next release-confidence direction

- Timestamp: 2026-02-19T16:41:38.7698879+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Which direction should be prioritized next to make release E2E more
  convincing?
- Alternatives:
  - Expand full happy-path release blocking to all three platforms first.
  - Replace fallback-only validation by adding Linux CoW/btrfs profile for existing
    blocking scenarios.
  - Add Liquibase release-blocking scenarios first.
- Decision: Prioritize Linux CoW/btrfs validation for existing release-blocking
  scenarios (`hp-psql-chinook`, `hp-psql-sakila`) before platform or Liquibase
  expansion.
- Rationale: This closes the biggest behavior gap between test path and real
  snapshot path while keeping runtime and scenario surface stable.

## Decision Record 2: workflow shape for btrfs coverage

- Timestamp: 2026-02-19T16:41:38.7698879+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should btrfs coverage be integrated into release-local CI?
- Alternatives:
  - Add a separate dedicated Linux btrfs job outside current happy-path matrix.
  - Expand current Linux happy-path job with matrix axis
    `snapshot_backend: [copy, btrfs]`.
  - Replace current copy profile entirely with btrfs-only runs.
- Decision: Expand current Linux happy-path E2E with matrix axis
  `snapshot_backend: [copy, btrfs]`, reusing same scenario catalog.
- Rationale: Matrix keeps workflow structure simple, preserves baseline copy
  profile, and makes backend coverage explicit per scenario.
