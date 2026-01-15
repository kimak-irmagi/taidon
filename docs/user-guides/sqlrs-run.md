# sqlrs run

## Overview

`sqlrs run` executes an external command against an **existing database instance**.

Unlike `prepare`, `run` **never constructs or restores database state**.
It only consumes an already available instance and injects its DSN into
the executed command.

`run` is designed to integrate sqlrs with arbitrary tools, applications,
and test runners.

---

## Terminology

- **State** — an immutable database state produced by `prepare`.
- **Instance** — a mutable database instance derived from a state.
- **DSN** — a connection string uniquely identifying an instance.

---

## Command Syntax

```text
sqlrs run[:kind] [OPTIONS] -- <command> [args...]
```

Where:

- `:kind` defines how the DSN is passed to the command.
- `<command>` is executed verbatim after `--`.
- `OPTIONS` control instance resolution.

---

## run[:kind] Concept

`run[:kind]` defines:

1. **DSN injection mechanism**
2. Optional **liveness tracking** for the instance

Built-in kinds include:

- `run:psql`
- `run:pgbench`

Additional run kinds may be declared declaratively in
`.sqlrs/config.yaml`.

---

## DSN Injection

Each `run:<kind>` specifies how the instance DSN is provided to the command.

Common mechanisms include:

- Environment variables
- Command-line argument rewriting
- Wrapper executables

Examples:

- `run:psql` injects DSN via `PGDATABASE`, `PGHOST`, etc.
- `run:pgbench` injects DSN via command-line flags

The exact behavior is defined by the selected `run:<kind>`.

---

## Instance Resolution

Before executing the command, `run` must resolve a target instance.

Resolution order:

1. Instance produced by a preceding `prepare` in the same invocation
2. `--dsn <dsn>`
3. `--instance <id>`

If resolution fails, `run` terminates with an error.

---

## Ephemeral Pipeline Usage

```bash
sqlrs prepare:psql init.sql run:psql -- -c "select 1"
```

- A temporary instance is created by `prepare`
- `run` executes against it
- Instance is typically discarded afterwards

---

## Instance Lifetime and Liveness

`run` itself does not define instance lifetime.

Depending on configuration:

- Ephemeral instances may be removed after command exit

Some `run:<kind>` implementations may:

- Extend instance lease while the process is running
- Perform health checks before execution

These behaviors are **implementation-specific** and not guaranteed by `run`.

---

## Error Handling

`sqlrs run` may fail with the following errors:

- **Instance not found**
  - Instance was never created or already purged

- **Expired instance**
  - Instance exists but is no longer active

- **Invalid DSN**
  - DSN format is invalid or unreachable

- **Execution failure**
  - The invoked command exited with non-zero status

When possible, errors include hints such as:

```text
Hint: run sqlrs prepare:psql ... to create the instance.
```

---

## Options

```text
--dsn <dsn>           Explicit DSN to use
--instance <id>       Target a specific instance
```

At most one of these options may be specified.

---

## Output

`run` forwards:

- stdout and stderr of the executed command
- exit code of the executed command

`run` itself produces no additional output on success.

---

## Examples

### Run psql against an ephemeral instance

```bash
sqlrs prepare:psql init.sql run:psql -- -c "select count(*) from users"
```

---

### Run with explicit DSN

```bash
sqlrs run --dsn "$DSN" -- pgbench -c 10 -T 30
```

---

## Guarantees

- `run` never modifies or creates states.
- `run` never restores missing instances.
- Instance resolution is explicit and deterministic.
- Command execution semantics are fully transparent.
