# sqlrs CLI Contract (Draft)

This document defines a **preliminary user-facing CLI contract** for `sqlrs`.
It is intentionally incomplete and iterative, similar in spirit to early `git` or `docker` CLI designs.

The goal is to:
- establish a stable *mental model* for users,
- define command namespaces and responsibilities,
- guide internal API and UX decisions.

---

## 0. Design Principles

1. **Canonical CLI name**: `sqlrs`
2. **Subcommand-based interface** (git/docker style)
3. **Explicit state over implicit magic**
4. **Composable commands** (plan → apply → run → inspect)
5. **Machine-friendly output by default** (JSON where applicable)
6. **Human-friendly summaries** when run interactively

---

## 1. High-Level Mental Model

From the user’s point of view, `sqlrs` manages:

- **states**: immutable database snapshots
- **sandboxes**: live, temporary databases created from states
- **plans**: ordered sets of changes to apply (e.g., Liquibase changesets)
- **runs**: executions of plans, scripts, or commands

```
state  --(materialize)-->  sandbox  --(run/apply)-->  new state
```

---

## 2. Command Groups (Namespaces)

```
sqlrs
 ├─ init
 ├─ plan
 ├─ migrate
 ├─ run
 ├─ state
 ├─ sandbox
 ├─ cache
 ├─ inspect
 ├─ tag
 ├─ config
 └─ system
```

Not all groups are required in MVP.

---

## 3. Core Commands (MVP)

### 3.1 `sqlrs init`

Initialize a sqlrs project in the current directory.

```bash
sqlrs init
```

Creates:
- `.sqlrs/` directory
- default config
- links to migration sources (e.g., Liquibase changelog)

---

### 3.2 `sqlrs plan`

Compute the execution plan without applying changes.

```bash
sqlrs plan [options]
```

Purpose:
- show pending changes
- compute hash horizon
- display potential cache hits

Common flags:

```
--format=json|table
--contexts=<ctx>
--labels=<expr>
--dbms=<db>
```

Example output (table):

```
STEP  TYPE        HASH        CACHED  NOTE
0     changeset  ab12…cd     yes     snapshot found
1     changeset  ef34…56     no      requires execution
2     changeset  ?           n/a     volatile
```

---

### 3.3 `sqlrs migrate`

Apply migrations step-by-step with snapshotting and caching.

```bash
sqlrs migrate [options]
```

Behavior:
- uses `plan`
- rewinds via cache where possible
- materializes sandboxes as needed
- snapshots after each successful step

Important flags:

```
--mode=conservative|investigative
--dry-run
--max-steps=N
```

---

### 3.4 `sqlrs run` (PoC form)

Run an arbitrary command or script against a sandbox. In the PoC we use a single
invocation with step flags.

```bash
sqlrs --run -- <command>
sqlrs --prepare -- <command> --run -- <command>
```

Examples:

```bash
sqlrs --run -- psql -c "SELECT count(*) FROM users"
sqlrs --prepare -- psql -f ./schema.sql --run -- pytest
```

Behavior:
- starts one sandbox per invocation
- executes `prepare` (if provided) then `run`
- snapshots after successful `prepare` into `workspace/states/<state-id>`
- stops on the first error with a non-zero exit code
- `--prepare` and `--run` are reserved and cannot appear inside commands
- only one `prepare` and one `run` per invocation

---

## 4. State and Sandbox Management

### 4.1 `sqlrs state`

Inspect and manage immutable states.

```bash
sqlrs state list
sqlrs state show <state-id>
```

---

### 4.2 `sqlrs sandbox`

Manage live sandboxes.

```bash
sqlrs sandbox list
sqlrs sandbox open <id>
sqlrs sandbox destroy <id>
```

---

## 5. Tagging and Discovery

### 5.1 `sqlrs tag`

Attach human-friendly metadata to states.

```bash
sqlrs tag add <state-id> --name v1-seed
sqlrs tag add --nearest --name after-schema
sqlrs tag list
```

Tags:
- do not change state identity
- influence cache eviction

---

## 6. Cache Control (Advanced)

### 6.1 `sqlrs cache`

Inspect and influence cache behavior.

```bash
sqlrs cache stats
sqlrs cache prune
sqlrs cache pin <state-id>
```

---

## 7. Inspection and Debugging

### 7.1 `sqlrs inspect`

Inspect plans, runs, and failures.

```bash
sqlrs inspect plan
sqlrs inspect run <run-id>
sqlrs inspect failure <state-id>
```

---

## 8. Configuration

### 8.1 `sqlrs config`

View and modify configuration.

```bash
sqlrs config get
sqlrs config set key=value
```

---

## 9. Output and Scripting

- Default output: human-readable
- `--json`: machine-readable
- Stable schemas for JSON output

Designed for CI/CD usage.

---

## 10. Input Sources (Local Paths, URLs, Remote Uploads)

Wherever the CLI expects a file or directory, it accepts:

- local path (file or directory)
- public URL (HTTP/HTTPS)
- server-side `source_id` (previous upload)

Behavior depends on target:

- local engine + local path: pass path directly
- remote engine + public URL: pass URL directly
- remote engine + local path: upload to source storage (chunked) and pass `source_id`

This keeps `POST /runs` small and enables resumable uploads for large projects.

---

## 11. Compatibility and Extensibility

- Liquibase is treated as an external planner/executor
- CLI does not expose Liquibase internals directly
- Future backends (Flyway, raw SQL, custom planners) fit into the same contract

---

## 12. Non-Goals (for this CLI contract)

- Full parity with Liquibase CLI options
- Interactive TUI
- GUI bindings

---

## 13. Open Questions

- Should we allow multiple prepare/run steps per invocation?
- Should `plan` be implicit in `migrate` or always explicit?
- How much state history should be shown by default?
- Should destructive operations require confirmation flags?

---

## 14. Philosophy

`sqrls` (sic) is not a database.

It is a **state management and execution engine** for databases.

The CLI should make state transitions explicit, inspectable, and reproducible.

