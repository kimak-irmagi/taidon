# sqlrs release happy-path E2E

## Overview

This guide defines the maintainer workflow for release validation with
happy-path end-to-end tests in GitHub Actions.

Main rule:

- Build release candidate artifacts once.
- Validate those exact artifacts in clean environments.
- Promote the same artifacts to GA release only after green E2E.

---

## CLI Surface Impact

No new end-user CLI syntax is introduced by this design.

E2E uses existing commands and examples, including existing init backend flag:

- `sqlrs init local --snapshot <auto|btrfs|overlay|copy>`

---

## Selected Direction (2026-02-19)

The next confidence step is to keep the same release-blocking happy-path
scenarios (`hp-psql-chinook`, `hp-psql-sakila`) and add a CoW-oriented profile
on Linux by running them with `--snapshot btrfs`.

This improves release confidence for the real snapshot path, not only fallback
copy path.

---

## Selected Direction (2026-03-03)

Add a Liquibase-based happy-path scenario sourced from `examples/liquibase/`.
This catches regressions in `prepare:lb` and validates that the engine can
accept DB connections from the host-side Liquibase process (not only from
in-container `psql`).

Initial scope:

- platform: Linux only (release-gated);
- runtime: Docker (same as existing scenarios);
- scenario id: `hp-lb-jhipster` (JHipster sample changelog);
- snapshot backend: `copy` only (btrfs is covered by existing psql scenarios).

Runner prerequisites (Linux):

- `liquibase` available on PATH (in CI we provide it as a small wrapper that executes `liquibase/liquibase:latest` via Docker, plus a downloaded PostgreSQL JDBC driver jar for connectivity).

---

## Tagging Contract

Use two tag classes:

- `vX.Y.Z-rc.N` for release candidate validation.
- `vX.Y.Z` for final promotion.

`RC` tags trigger build and E2E.  
`GA` tags trigger promotion of already validated artifacts.

---

## Release Validation Flow

1. Push RC tag `vX.Y.Z-rc.N`.
2. Build bundles for target platforms/architectures.
3. Run Linux happy-path E2E against downloaded RC bundles using matrix:
   `scenario x snapshot_backend`.
4. Publish RC assets as pre-release only if E2E passed.
5. Push GA tag `vX.Y.Z` to promote the exact RC assets.

Promotion must verify checksums before publishing.
Promotion must also verify that `release-manifest.json` `source_sha` matches the
GA tag commit SHA.

---

## Scenario Source

Happy-path scenarios are sourced from `examples/` with stable scripts and
queries.
In CI, datasets are fetched in locked mode (`pnpm fetch:sql --lock`) before
executing scenarios.

Scenario catalog lives in:

- `test/e2e/release/scenarios.json`

Each scenario should define:

- Example path.
- Preparation command.
- Run command.
- Expected output snapshot path.
- Output normalization rules.

Linux runner matrix adds one more runtime axis in workflow configuration:

- `snapshot_backend`: `copy`, `btrfs`.

The same `scenarios.json` catalog is reused for both backends.

Release workflow also includes a macOS podman probe cell:

- runner: `macos-15-intel`;
- scenario: `hp-psql-chinook` with `snapshot_backend=copy`;
- runtime mode set via `sqlrs config set container.runtime podman`;
- `prepare+run` flow is executed twice in the same initialized workspace.
- this cell is release-blocking: podman machine startup failure fails RC gating.

---

## Linux CoW Matrix Contract

Release workflow should execute Linux happy-path E2E with:

- scenario axis: `hp-psql-chinook`, `hp-psql-sakila`;
- snapshot backend axis: `copy`, `btrfs`.

Expected init behavior per matrix cell:

- `copy`: `sqlrs init local --snapshot copy`.
- `btrfs`: `sqlrs init local --snapshot btrfs` (and btrfs-compatible store
  parameters when needed by the runner environment).

Blocking policy:

- both Linux backend profiles are required for RC publication;
- Windows/macOS smoke checks remain required unchanged.

---

## Liquibase Scenario Contract (hp-lb-jhipster)

The goal of `hp-lb-jhipster` is to validate the Liquibase integration end-to-end
using a non-trivial changelog tree (includes + CSV loads).

Source:

- `examples/liquibase/jhipster-sample-app`

Flow:

1. `sqlrs prepare:lb --image postgres:17 -- update --changelog-file config/liquibase/master.xml`
2. `sqlrs run:psql -- -At -f .e2e-query.sql`

The scenario query should validate that Liquibase applied changes by checking a
small set of stable artifacts, for example:

- `databasechangelog` table exists;
- `jhi_user` table exists;
- at least one row in `jhi_user` (from `loadData` CSVs).

This scenario requires host-side Liquibase execution, so it also validates that
PostgreSQL accepts host connections through the published port (Docker bridge
source IP).

---

## Comparison Rules

Output comparison must normalize unstable fields before diff:

- timestamps;
- process IDs, random ports, generated IDs;
- host-specific absolute paths;
- line order when explicitly non-deterministic.

Normalized output should be compared against committed golden files.

---

## Required Artifacts on Failure

On E2E failure, always upload:

- CLI stdout/stderr;
- engine logs and events;
- normalized output and unified diff against golden;
- scenario metadata and environment info.
