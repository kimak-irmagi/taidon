# ADR: add Liquibase happy-path to release E2E

Status: Accepted
Date: 2026-03-03

## Decision Record 1: add Liquibase scenario to release E2E gating

- Timestamp: 2026-03-03T16:44:07.671126+07:00
- User: @evilguest
- Agent: Codex CLI (GPT-5.2)
- Question: Should Liquibase happy-path become release-blocking now?
- Alternatives:
  - Keep Liquibase non-blocking and rely on psql-only scenarios.
  - Add a Liquibase scenario to the release-gated Linux happy-path E2E.
- Decision: Add `hp-lb-jhipster` as a release-gated Linux happy-path E2E scenario.
- Rationale: Liquibase integration (`prepare:lb`) is critical and a recent host
  connectivity regression (pg_hba.conf) should be caught by CI before release.

## Decision Record 2: scenario scope and Liquibase distribution

- Timestamp: 2026-03-03T16:44:07.671126+07:00
- User: @evilguest
- Agent: Codex CLI (GPT-5.2)
- Question: What scope should the Liquibase scenario cover, and how should Liquibase be provided in CI?
- Alternatives:
  - Run the scenario on `copy+btrfs` snapshot backends in the Linux matrix.
  - Run the scenario on `copy` only (keep matrix size/time bounded).
  - Install pinned Liquibase CLI binaries (version+checksum) on the runner.
  - Install Liquibase via OS package manager (less deterministic).
  - Provide `liquibase` via a PATH wrapper that executes `liquibase/liquibase:latest` using Docker.
- Decision: Run `hp-lb-jhipster` on Linux with `snapshot_backend=copy` only, and
  provide `liquibase` via a wrapper around `liquibase/liquibase:latest`.
- Rationale: `btrfs` behavior is already validated by existing psql scenarios,
  while using `latest` catches upstream regressions from Liquibase itself.
