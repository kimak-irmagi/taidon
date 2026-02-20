# sqlrs Windows WSL happy-path probe

## Purpose

This workflow is an experimental pre-integration check to validate that
`hp-psql-chinook` happy-path E2E can run on standard `windows-latest` GitHub
hosted runners using WSL2.

It is intentionally separate from `release-local.yml` until the path is stable.

## Workflow

Workflow file:

- `.github/workflows/e2e-windows-wsl-probe.yml`

Execution model:

1. Checkout repository on `windows-latest`.
2. Build Linux `sqlrs` and `sqlrs-engine` binaries from source.
3. Initialize Docker on the Windows host via `docker/setup-docker-action`.
4. Install/setup WSL distro (`Ubuntu-24.04`) via `Vampire/setup-wsl`.
5. Download `examples/chinook/Chinook_PostgreSql.sql` on host with locked
   sha256 verification.
6. Inside WSL:
   - ensure Docker daemon is available;
   - run `sqlrs init local --snapshot copy`;
   - run `prepare:psql` + `run:psql` for `examples/chinook`.
7. Normalize stdout and compare with committed golden output:
   `test/e2e/release/hp-psql-chinook/golden.txt`.
8. Upload diagnostics artifacts for post-failure analysis.

## Trigger

Temporary triggers during probe stage:

- `push` to any branch except `main`;
- `workflow_dispatch` (manual run).

Input:

- `scenario` (currently fixed choice: `hp-psql-chinook`)

## Output Artifacts

Diagnostics are uploaded under:

- `e2e-windows-wsl-probe-hp-psql-chinook`

Including:

- init and flow command logs;
- raw/normalized stdout/stderr;
- golden diff;
- Docker daemon log from WSL;
- engine state/log files when available.
