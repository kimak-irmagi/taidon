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

## Decision Record 3: strict Linux btrfs init semantics for release confidence

- Timestamp: 2026-02-19T19:27:10.0396864+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs init local --snapshot btrfs` behave on Linux so
  that btrfs mode is guaranteed instead of silently degrading to copy mode?
- Alternatives:
  - Keep current behavior: write config only and rely on runtime fallback.
  - Fail when target path is not already btrfs, requiring manual setup.
  - Reuse existing btrfs store when available; otherwise provision loopback
    image, format to btrfs, mount it to store path, and fail hard on errors.
- Decision: Use strict behavior on Linux for `snapshot=btrfs`: if target store
  is already btrfs (root fs or mounted device), continue; otherwise
  automatically provision and mount loopback btrfs (or device path when
  explicitly requested), and fail initialization on any provisioning error.
- Rationale: This aligns Linux behavior with the "requested btrfs means real
  btrfs" expectation and prevents false-green release validation through silent
  copy fallback.

## Decision Record 4: explicit shared btrfs-init policy and split boundaries

- Timestamp: 2026-02-19T19:38:11.1387483+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should Linux and Windows btrfs init be unified, and what should be
  shared explicitly?
- Alternatives:
  - Keep implementations fully independent with only implicit behavioral overlap.
  - Fully unify provisioning implementation across Linux and Windows/WSL.
  - Share policy and invariants explicitly, keep platform provisioning logic
    separate.
- Decision: Use partial unification: keep a shared explicit policy/invariants
  layer ("btrfs requested => real btrfs or fail"), while preserving separate
  low-level provisioning code paths for Linux and Windows/WSL.
- Rationale: Platform command stacks and failure modes differ substantially, but
  policy and expected outcomes are common and should be centralized and
  documented to avoid drift.
