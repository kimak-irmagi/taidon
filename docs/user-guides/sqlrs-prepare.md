# sqlrs prepare

## Overview

`sqlrs prepare` is the only command that can **deterministically construct or restore**
a database state in sqlrs.

A `prepare:psql` invocation:

1. Identifies an immutable **state** based on its arguments.
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
sqlrs prepare:psql [--image <image-id>] [--] [psql-args...]
```

Where:

- `--image <image-id>` overrides the base Docker image.
- `psql-args` are passed to `psql` and fully describe how the state is produced.

If `--` is omitted, all remaining arguments are treated as `psql-args`.
To pass `psql` flags that would clash with sqlrs flags (for example `-v`),
use `--` explicitly.

---

## `prepare:psql` Concept

`prepare:psql` defines:

- how the database state is constructed,
- which inputs participate in state identification,
- how the executor connects to the database during preparation.

---

## psql Argument Handling

`prepare:psql` aims to match `psql` semantics closely. All `psql-args` are passed
verbatim to `psql` with two enforced defaults for determinism:

- `-X` (ignore `~/.psqlrc`)
- `-v ON_ERROR_STOP=1`

If a user-provided argument conflicts with these enforced defaults, `prepare`
fails with an error.

Connection arguments are rejected because sqlrs supplies the connection to the
prepared instance:

- `-h`, `--host`
- `-p`, `--port`
- `-U`, `--username`
- `-d`, `--dbname`, `--database`

### SQL input sources

- `-f`, `--file <path>`: SQL script file. Relative paths are resolved from the CLI working
  directory and sent as absolute paths; files must live under the workspace root.
- `-c`, `--command <sql>`: inline SQL string.
- `-f -`: read SQL from stdin; sqlrs reads stdin and passes it to the engine.

All inputs above participate in state identification.

---

## Base Image Selection

The base Docker image id is resolved in this order:

1. `--image <image-id>` command-line flag
2. Workspace config (`.sqlrs/config.yaml`, `dbms.image`)
3. Global config (`$XDG_CONFIG_HOME/sqlrs/config.yaml`, `dbms.image`)

The image id is treated as an opaque value and passed to Docker as-is.
If no image id can be resolved, `prepare` fails.

Config key:

```yaml
dbms:
  image: postgres:17
```

When `-v/--verbose` is set, sqlrs prints the resolved image id and its source.

---

## Local Execution (MVP)

For local profiles, the engine performs real execution:

- Requires Docker running locally; `psql` is executed inside the container.
- Images must expose `PGDATA` at `/var/lib/postgresql/data` and allow trust auth
  (`POSTGRES_HOST_AUTH_METHOD=trust`).
- State data is stored under `<StateDir>/state-store` (outside containers).
- Each task snapshots the DB state; the engine uses OverlayFS-based copy-on-write
  when available and falls back to full copy.
- The prepare container stays running after the job; the instance is recorded as
  warm and a future `sqlrs run` will decide when to stop it.
- When `-f/--file` inputs are present, the engine mounts the workspace scripts
  root into the container and rewrites file arguments to the container path.

---

## State Identification

A **state** is identified by a fingerprint computed from:

- `prepare kind`
- `base image id`
- normalized `prepare arguments`
- hashes of all input sources (files, inline SQL, stdin)
- sqlrs engine version

Formally:

```text
state_id = hash(
  prepare_kind +
  base_image_id +
  normalized_prepare_args +
  normalized_input_hashes +
  engine_version
)
```

### Normalization Rules

- File paths are resolved to absolute paths.
- Argument ordering is preserved.
- `psql` arguments are included verbatim, with enforced defaults applied.

If any participating input changes, a **new state** is produced.

---

## Instance Creation Semantics

Each `prepare` invocation creates a **new ephemeral instance** derived from the
identified state.

### Instance Modes

- **Ephemeral instance**

  - Created for each invocation.
  - Intended for short-lived usage.
  - Typically removed automatically.

---

## Error Conditions

`sqlrs prepare` may fail with the following errors:

- **Invalid inputs**

  - Missing files.
  - Invalid arguments for the selected `prepare:psql`.

- **Executor failure**

  - Preparation tool exited with non-zero status.

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

## Examples

### Ephemeral preparation

```bash
sqlrs prepare:psql -- -f ./init.sql
```

Creates a new instance derived from the state produced by `init.sql`.

---

### Override base image

```bash
sqlrs prepare:psql --image postgres:17 -- -f ./init.sql
```

Creates the instance using the specified base image.

---

### Use stdin

```bash
cat ./init.sql | sqlrs prepare:psql -- -f -
```

---

## Guarantees

- `prepare` is idempotent with respect to state identification.
- State objects are immutable once created.
