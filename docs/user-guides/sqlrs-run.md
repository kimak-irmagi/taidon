# sqlrs run

## Overview

`sqlrs run` executes an external command against an **existing database instance**.

Unlike `prepare`, `run` **never constructs or restores database state**.
It only consumes an already available instance and injects its DSN into
the executed command. If the instance container is missing but the runtime
data directory is still present, `run` may recreate the container using
the preserved runtime data before executing the command.

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
sqlrs run[:kind] [OPTIONS] [-- <command> ] [args...]
```

Where:

- `:kind` defines how the DSN is passed to the command.
- `<command>` is executed verbatim after `--` (optional).
- `OPTIONS` control instance resolution.

If `<command>` is omitted, `run:<kind>` uses its default command. In that case,
`--` may be omitted as well, and any remaining arguments apply to the default
command.

---

## run[:kind] Concept

`run[:kind]` defines:

1. **DSN injection mechanism**
2. The **default command**, if none is provided
3. Optional **liveness tracking** for the instance

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

- `run:psql` injects DSN as a positional connection string
- `run:pgbench` injects DSN via command-line flags

The exact behavior is defined by the selected `run:<kind>`.

### `run:psql` injection rules

- Default command: `psql`.
- The command runs **inside the instance container**, same as `prepare:psql`.
- DSN is passed as a positional argument (connection string).
- If the user supplies conflicting connection args (for example `-h/-p/-U/-d`
  or a positional connection string), `run:psql` fails with an error.
- Each `-c`, `-f`, and `-` (stdin) source is executed as a **separate** `psql`
  invocation to match upstream transaction semantics. When `-f -` is used,
  stdin can be consumed only once.

### `run:pgbench` injection rules

- Default command: `pgbench`.
- The command runs **inside the instance container**, same as `prepare:psql`.
- DSN is passed via `pgbench` args (`-h/-p/-U/-d`).
- If the user supplies conflicting connection args (`-h/-p/-U/-d`), `run:pgbench`
  fails with an error.

---

## Instance Resolution

Before executing the command, `run` must resolve a target instance.

Resolution order:

1. Instance produced by a preceding `prepare` in the same invocation
2. `--instance <id>`

If resolution fails, `run` terminates with an error.

If both `--instance` and a preceding `prepare` are present, `run` fails with an
explicit ambiguity error.

`--instance` accepts either an instance id or a name. If multiple candidates are
found, `run` fails with an ambiguity error.

---

## Ephemeral Pipeline Usage

```bash
sqlrs prepare:psql -- -f init.sql run:psql -- -c "select 1"
```

- A temporary instance is created by `prepare`
- `run` executes against it
- Instance is discarded immediately after `run` finishes

---

## Instance Lifetime and Liveness

`run` itself does not define instance lifetime.

Depending on configuration:

- Ephemeral instances created by `prepare` remain warm and await a connection.
- In a composite `prepare ... run` invocation, the instance is removed
  immediately after `run` finishes.

Connection-count based cleanup (stop after the first client disconnects) is a
desired behavior but is not implemented yet.

Some `run:<kind>` implementations may:

- Extend instance lease while the process is running
- Perform health checks before execution

These behaviors are **implementation-specific** and not guaranteed by `run`.

---

## Instance Recovery (Missing Container)

If an instance exists in the registry but its container is missing (for example,
Docker was restarted or the container was removed externally), `run` attempts to
recreate the container **using the instance runtime data directory**.

Rules:

- The runtime data directory must exist and be readable.
- The container is recreated from `runtime_dir` and the instance's `image_id`.
- The instance `runtime_id` is updated to the new container id.
- The `runtime_dir` path is preserved (it is not regenerated).
- If `runtime_dir` is missing, `run` fails with an error (no fallback to state).

This recovery is intended to be transparent to the CLI user and should not
change command semantics, only reduce failures caused by missing containers.

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

- **Missing runtime data**
  - Instance exists but `runtime_dir` is missing or unreadable, and container
    recovery is not possible

When possible, errors include hints such as:

```text
Hint: run sqlrs prepare:psql ... to create the instance.
```

---

## Options

```text
--instance <id>       Target a specific instance
```

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
sqlrs prepare:psql -- -f init.sql run:psql -- -c "select count(*) from users"
```

---

### Run with explicit instance

```bash
sqlrs run:pgbench --instance my-instance -- -c 10 -T 30
```

---

## Guarantees

- `run` never modifies or creates states.
- `run` may recreate a missing container when `runtime_dir` exists, but it never
  rebuilds state from scratch.
- Instance resolution is explicit and deterministic.
- Command execution semantics are fully transparent.
