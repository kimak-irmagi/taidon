# sqlrs prepare (psql)

## Overview

`sqlrs prepare:psql` builds a deterministic database state using `psql`.

A `prepare:psql` invocation:

1. Identifies an immutable **state** based on its arguments.
2. Ensures this state exists (by reusing or building it).
3. Creates a mutable **instance** derived from this state.
4. Returns a DSN pointing to that instance.

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

## Local Execution (MVP)

For local profiles, the engine performs real execution:

- Requires Docker running locally; `psql` is executed inside the container.
- Images must expose `PGDATA` at `/var/lib/postgresql/data` and allow trust auth
  (`POSTGRES_HOST_AUTH_METHOD=trust`).
- State data is stored under `<StateDir>/state-store` (outside containers).
- Each task snapshots the DB state; the engine prefers OverlayFS on Linux, can use
  btrfs subvolume snapshots when configured, and falls back to full copy.
- The prepare container stays running after the job; the instance is recorded as
  warm and a future `sqlrs run` will decide when to stop it.
- When `-f/--file` inputs are present, the engine mounts the workspace scripts
  root into the container and rewrites file arguments to the container path.

---

## State Identification

A **state** is identified by a fingerprint computed from:

- `prepare kind`
- resolved `base image id` (digest)
- normalized `prepare arguments`
- hashes of all input sources (files, inline SQL, stdin)
- sqlrs engine version

Formally:

```text
state_id = hash(
  prepare_kind +
  base_image_id_resolved +
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

- `prepare:psql` is idempotent with respect to state identification.
- State objects are immutable once created.
