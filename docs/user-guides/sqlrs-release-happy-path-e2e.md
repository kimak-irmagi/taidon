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

E2E uses existing commands and examples.

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
3. Run happy-path E2E against downloaded RC bundles.
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
