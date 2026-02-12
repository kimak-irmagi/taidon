# sqlrs prepare

## Overview

`sqlrs prepare` is the only command that can **deterministically construct or restore**
a database state in sqlrs.

A `prepare` invocation:

1. Identifies an immutable **state** based on its arguments and inputs.
2. Ensures this state exists (by reusing or building it).
3. Creates a mutable **instance** derived from this state.
4. Returns a DSN pointing to that instance.

All reproducibility guarantees in sqlrs rely on `prepare`.

---

## Terminology

- **State** - an immutable database state produced by a deterministic preparation process.
- **Instance** - a mutable copy of a state; all database modifications happen here.

---

## Command Syntax

```text
sqlrs prepare:<kind> [--image <image-id>] [--] [tool-args...]
```

Where:

- `:<kind>` selects the preparation variant (for example, `psql`, `lb`).
- `--image <image-id>` overrides the base DB image.
- `tool-args` are forwarded to the underlying tool for the selected kind.

### Variant docs

- `psql`: [`sqlrs-prepare-psql.md`](sqlrs-prepare-psql.md)
- `lb`: [`sqlrs-prepare-liquibase.md`](sqlrs-prepare-liquibase.md)

---

## Base Image Selection (Common)

The base Docker image id is resolved in this order:

1. `--image <image-id>` command-line flag
2. Workspace config (`.sqlrs/config.yaml`, `dbms.image`)
3. Global config (`$XDG_CONFIG_HOME/sqlrs/config.yaml`, `dbms.image`)

If the resolved image id does not include a digest, the engine resolves it to a
canonical digest (for example `postgres:15@sha256:...`) before planning or
execution. If the digest cannot be resolved, `prepare` fails.

When a digest resolution is required, the job includes a `resolve_image` task
before any planning or execution.

Config key:

```yaml
dbms:
  image: postgres:17
```

When `-v/--verbose` is set, sqlrs prints the resolved image id and its source.

---

## Local Execution (MVP)

For local profiles, the engine performs real execution:

- Requires Docker running locally; the selected tool runs inside a container.
- State data is stored under `<StateDir>/state-store` (outside containers).
- Each task snapshots the DB state; the engine prefers OverlayFS on Linux, can use
  btrfs subvolume snapshots when configured, and falls back to full copy.
- The prepare container stays running after the job; the instance is recorded as
  warm and a future `sqlrs run` will decide when to stop it.

---

## State Identification

State identification depends on the **prepare kind** and is documented in each
variant guide.

---

## Error Conditions

`sqlrs prepare` may fail with the following errors:

- **Invalid inputs**
  - Missing files required by the selected variant.
  - Invalid arguments for the selected `prepare:<kind>`.

- **Executor failure**
  - Underlying tool exited with non-zero status.

- **Engine errors**
  - Storage backend unavailable.
  - Insufficient permissions.

All errors are reported before any mutable instance is exposed.

---

## Output

On success, `prepare` prints the DSN of the selected instance to stdout.
With `-v/--verbose`, extra details (including image source) are printed to stderr.

Example:

```text
DSN=postgres://...
```

This DSN uniquely identifies the instance and can be consumed by `sqlrs run`
or external applications.

---

## Job Monitoring (Events-First)

`sqlrs prepare` monitors engine progress through the job events stream rather
than periodic status polling.

### Event Stream Rules

- The CLI uses the `events_url` returned by `POST /v1/prepare-jobs`.
- The events stream is newline-delimited JSON (`application/x-ndjson`).
- The engine emits task/status events plus log events from tool execution.
- During long-running tasks, the engine repeats the last task event with a new
  timestamp when no new events appear for ~500ms, so the CLI can keep showing
  progress even if the underlying system is quiet.
- The CLI reads events until the stream is exhausted:
  - If the response status is 200 and `Content-Length` is present, the stream is
    considered complete after the declared byte length is fully read.
  - If the response is any 4xx status, the stream is considered complete and
    the command fails.
- If the stream ends without a definitive job outcome (`succeeded` or `failed`),
  the command fails with an error.

### Status Validation

When a status event is received (queued, running, succeeded, failed), the CLI
re-fetches the job status via `GET /v1/prepare-jobs/{jobId}` to confirm the
final result. The job is considered complete only when the status endpoint
returns `succeeded` or `failed`.

### Progress UX

Progress events are printed to stderr. By default, the CLI rewrites the same
line and shows a spinner when events repeat. It also includes the `message` from
the latest event if present.

With `--verbose`, each event is printed on a new line (no overwriting).

---

## Guarantees

- `prepare` is idempotent with respect to state identification.
- State objects are immutable once created.
