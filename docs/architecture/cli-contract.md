# sqlrs CLI Contract (Draft)

This document defines a **preliminary user-facing CLI contract** for `sqlrs`.
It is intentionally incomplete and iterative, similar in spirit to early `git` or `docker` CLI designs.

The goal is to:

- establish a stable _mental model_ for users,
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

- **states**: immutable database states produced by a deterministic preparation process
- **instances**: mutable copies of states; all database modifications happen here
- **plans**: ordered sets of changes to apply (e.g., Liquibase changesets)
- **runs**: executions of plans, scripts, or commands

```text
state  --(materialize)-->  instance  --(run/apply)-->  new state
```

---

## Command Shape Convention

Across sqlrs, commands generally follow this shape:

```text
sqlrs <verb>[:<kind>] [subject] [options] [-- <command>...]
```

- `<verb>` is the main command (`prepare`, `run`, `ls`, ...).
- `:<kind>` is an optional executor/adaptor selector (e.g., `prepare:psql`, `run:pgbench`).
- `subject` is optional and verb-specific (e.g., an instance id, a name, etc.).
- `-- <command>...` appears only for verbs that execute an external command (primarily `run`).

`sqlrs ls` itself does not use `:<kind>` and does not accept `-- <command>...`.

## ID Prefix Rules

Anywhere the CLI expects an id, users may supply a hex prefix (minimum 8 chars).
The CLI resolves the prefix case-insensitively and fails on ambiguity.

## 2. Command Groups (Namespaces)

```text
sqlrs
  init
  status
  ls
  rm
  prepare
  plan
  run
```

Not all groups are required in MVP.

---

## 3. Core Commands (MVP)

### 3.1 `sqlrs init`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-init.md`](../user-guides/sqlrs-init.md)

---

### 3.2 `sqlrs status`

Check local or remote engine health and report status details.

```bash
sqlrs status [options]
```

---

### 3.3 `sqlrs ls`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-ls.md`](../user-guides/sqlrs-ls.md)

---

### 3.4 `sqlrs rm`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-rm.md`](../user-guides/sqlrs-rm.md)
ID prefix support (implemented):

- Full ids are hex strings (instances: 32 chars, states: 64 chars).
- Any id argument accepts 8+ hex characters as a case-insensitive prefix.
- If the value is shorter than 8 or contains non-hex characters, it is treated as a name.
- If a prefix matches multiple ids, the command fails with an ambiguity error.
- Human output shortens ids to 12 characters by default; `--long` prints full ids.
- JSON output always uses full ids.

---

### 3.5 `sqlrs prepare`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-prepare.md`](../user-guides/sqlrs-prepare.md)

TODO (future):

- Add `prepare:liquibase` (alias: `prepare:lb`).
- Add named instances and name binding flags (`--name`, `--reuse`, `--fresh`, `--rebind`).
- Add async mode (`--watch/--no-watch`) with `prepare_id` and status URL output.

---

### 3.6 `sqlrs plan`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-plan.md`](../user-guides/sqlrs-plan.md)

---

### 3.7 `sqlrs run`

See the user guide for the authoritative, up-to-date command semantics:

- [`docs/user-guides/sqlrs-run.md`](../user-guides/sqlrs-run.md)

---

## 4. Output and Scripting

- Default output: human-readable
- `--json`: machine-readable
- Stable schemas for JSON output

Designed for CI/CD usage.

---

## 5. Input Sources (Local Paths, URLs, Remote Uploads)

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

## 6. Compatibility and Extensibility

- Liquibase is treated as an external planner/executor
- CLI does not expose Liquibase internals directly
- Future backends (Flyway, raw SQL, custom planners) fit into the same contract

---

## 7. Non-Goals (for this CLI contract)

- Full parity with Liquibase CLI options
- Interactive TUI
- GUI bindings

---

## 8. Open Questions

- Should we allow multiple prepare/run steps per invocation? (see user guides)
- Should `plan` be implicit in `migrate` or always explicit?
- How much state history should be shown by default?
- Should destructive operations require confirmation flags?

---

## 9. Philosophy

`sqrls` (sic) is not a database.

It is a **state management and execution engine** for databases.

The CLI should make state transitions explicit, inspectable, and reproducible.
