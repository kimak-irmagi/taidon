# sqlrs prepare

## Overview

`sqlrs prepare` is the only command that can **deterministically construct or restore**
a database state in sqlrs.

A `prepare:<kind>` invocation:

1. Identifies an immutable **state** based on its arguments.
2. Ensures this state exists (by reusing or building it).
3. Creates or selects a mutable **instance** derived from this state.
4. Returns a DSN pointing to that instance.

All reproducibility guarantees in sqlrs rely on `prepare`.

---

## Terminology

- **State** — an immutable database state produced by a deterministic preparation process.
- **Instance** — a mutable copy of a state; all database modifications happen here.
- **Name** — a stable user-defined handle bound to an instance and its originating state.

---

## Command Syntax

```text
sqlrs prepare:<kind> <prepare-args> [OPTIONS]
```

Where:

- `<kind>` defines the preparation method and executor.
- `<prepare-args>` fully describe how the state is produced.
- `OPTIONS` control instance creation and name binding.

---

## `prepare:<kind>` Concept

`prepare:<kind>` defines:

- how the database state is constructed,
- which inputs participate in state identification,
- how the executor connects to the database during preparation.

Examples of built-in kinds:

- `prepare:psql`
- `prepare:liquibase` (alias: `prepare:lb`)

Each `<kind>` defines its own argument semantics, but all kinds share
the same lifecycle and identity rules.

---

## State Identification

A **state** is identified by a fingerprint computed from:

- `prepare kind`
- normalized `prepare arguments`
- hashes of all derived input files (normalzed)
- sqlrs engine version

Formally:

```text
state_id = hash(
  prepare_kind +
  normalized_prepare_args +
  normalized_input_hashes +
  engine_version
)
```

### Normalization Rules

- File paths are resolved to absolute paths.
- Glob patterns are expanded deterministically.
- Argument ordering is normalized.
- Runtime-only flags (verbosity, progress output) are excluded.

If any participating input changes, a **new state** is produced.

---

## Instance Creation Semantics

Each `prepare` invocation results in selecting or creating a **mutable instance**
derived from the identified state.

### Default Behavior

| Condition              | Result                         |
| ---------------------- | ------------------------------ |
| `--name` not specified | New ephemeral instance         |
| `--name` specified     | Reuse existing instance if any |

### Instance Modes

- **Ephemeral instance**

  - Created when `--name` is not specified.
  - Intended for short-lived usage.
  - Typically removed automatically.

- **Named instance**
  - Created or reused when `--name <name>` is provided.
  - Persists independently of process lifetime.
  - Subject to TTL / garbage collection policies.

---

## Name Binding Rules

A name is bound to:

- the current `state fingerprint`
- a specific `instance` derived from that state

### Reuse vs Fresh

- `--reuse`  
  Reuse the existing instance bound to the name (default when `--name` is set).
  Issues warning whenever the instance doesn't exist.

- `--fresh`  
  Discard the existing instance and create a new one from the same state.

These options are **mutually exclusive**.

### State Mismatch Protection

If a name already exists and is bound to a **different state fingerprint**:

- `prepare` fails by default.
- This prevents silent reuse of outdated database states.

To explicitly change the meaning of a name, use:

```text
--rebind
```

This destroys the old instance and rebinds the name to the new state.
**Note** this also preserves the DSN used for the instance bound to this name.

---

## Flags and Defaults

```text
--name <name>     Bind the instance to a stable name
--reuse           Reuse existing named instance (default with --name)
--fresh           Create a new instance from the same state
--rebind          Explicitly change state binding for an existing name
```

### Default Resolution

| Condition          | Default Mode |
| ------------------ | ------------ |
| `--name` specified | `--reuse`    |
| `--name` omitted   | `--fresh`    |

---

## Error Conditions

`sqlrs prepare` may fail with the following errors:

- **State mismatch**

  - Name exists but is bound to a different state fingerprint.
  - Requires `--rebind` to proceed.

- **Invalid inputs**

  - Missing files.
  - Invalid arguments for the selected `prepare:<kind>`.

- **Executor failure**

  - Preparation tool exited with non-zero status.

- **Engine errors**
  - Storage backend unavailable.
  - Insufficient permissions.

All errors are reported before any mutable instance is exposed.

---

## Output

On success, `prepare` prints the DSN of the selected instance to stdout.

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
sqlrs prepare:psql ./init.sql
```

Creates a new instance derived from the state produced by `init.sql`.

---

### Named reusable instance

```bash
sqlrs prepare:psql ./init.sql --name devdb
```

Creates or reuses a persistent instance named `devdb`.

---

### Force clean rebuild

```bash
sqlrs prepare:psql ./init.sql --name devdb --fresh
```

Recreates the instance while keeping the same state.

---

### Rebinding a name to a new state

```bash
sqlrs prepare:psql ./init_v2.sql --name devdb --rebind
```

Changes the meaning of `devdb` to point to a new state.

---

## Guarantees

- `prepare` is idempotent with respect to state identification.
- State objects are immutable once created.
- Named instances are never silently reused across different states.
