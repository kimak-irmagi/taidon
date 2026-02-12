# sqlrs prepare (liquibase)

This document describes how `sqlrs prepare:lb` should work in the **local** deployment profile.

---

## Goals

- Delegate Liquibase changelog parsing/format handling to Liquibase itself (XML/YAML/JSON/formatted SQL).
- Keep prepare deterministic and cacheable.
- Avoid mounting the entire workspace when possible.
- Minimize new complexity in the CLI and engine.

## Non-goals (initially)

- Remote engine profiles (team/cloud).
- Custom Java extensions / classpath injection beyond standard Liquibase image contents.
- Container-based Liquibase execution (planned, not implemented yet).

---

## CLI Syntax (proposed)

```text
sqlrs prepare:lb [--image <db-image-id>]
                             -- <liquibase-args...>
```

### Flags

- `--image <db-image-id>` (optional): overrides the DB base image (same as `prepare:psql`).
- `liquibase-args...` (required): passed to Liquibase CLI after `--`.

### Config fallback

Liquibase executable path is resolved from config. Otherwise, `liquibase` is
resolved via PATH.

```yaml
liquibase:
  exec: C:\Program Files\Liquibase\liquibase.exe
```

---

## Local Execution Model

1. Engine creates (or reuses) a base state for the DB image (same as `prepare:psql`).
2. Engine launches **host Liquibase** (Windows executable) from WSL via interop.
3. User provides **Liquibase command line** after `--` (for example `update`).
4. Engine executes **plan** via `updateSQL`, inspects the resulting changesets and
   builds a fine-grained plan.
5. Engine executes the plan as a sequence of `update-count --count=1` steps, snapshotting after
   each changeset.
6. The prepared state is cached and a new instance is created.

### Planning vs execution tasks

- **Plan task**: run `liquibase updateSQL` and **parse its output** to compute the
  ordered list of changesets and the SQL for each changeset (no DB changes).
- **Prepare tasks**: execute changesets **one by one** with `liquibase update-count --count=1`,
  emitting events per task (stdout lines -> events, exit -> status event).

### Connection parameters

Connection arguments are **always** supplied by the engine and override defaults file values.
User-provided `--url`, `--username`, `--password`, `--classpath`, etc. are rejected.

---

## Path mapping (host Liquibase)

The CLI passes **WSL paths** to the engine. When Liquibase runs on the host, the
engine translates relevant arguments to **Windows paths**:

- `--changelog-file`
- `--defaults-file`
- `--searchPath`

This mapping is applied before Liquibase is executed, so it can resolve files on
the host filesystem.

The path mapper is **abstracted** to support future container execution (WSL paths
will be mapped to container mount paths instead of Windows paths).

---

## File resolution behavior (Liquibase)

Liquibase CLI resolves files relative to its **current working directory**, plus
classpath paths and a configured search path. When `--search-path` is set, it
overrides the default search locations. By default, Liquibase looks for a
`liquibase.properties` file in the directory where it is run.

**Proposed behavior in sqlrs**:

- The Liquibase **working directory** is the CLI working directory.
- sqlrs does **not** attempt to resolve includes itself. Liquibase handles it.

This delegates include resolution to Liquibase.

---

## Deterministic fingerprint (state id)

State identification is based on the **ordered list of Liquibase changesets**
returned by planning, plus the **parent state id**. The parent id is the task
input: for the first step it is the base **image id**, and for subsequent steps
it is the previous **state id**. The fingerprint is **content-based** rather
than path-based:

- `prepare kind` = `lb`
- `prev_state_id` (from the task input)
- ordered list of changesets as reported by Liquibase:
  - `changeset_hash` (preferred: Liquibase checksum when available; fallback:
    hash of SQL emitted for that changeset by `updateSQL`)
  - `id/author/path` are recorded for diagnostics but **do not** affect the fingerprint

If two different argument sets produce the same ordered changesets (including
per-changeset content hashes), sqlrs reuses the cached state for that chain.

---

## Content locking (atomicity)

To avoid plan drift, sqlrs locks all Liquibase inputs during each task that
computes or applies content:

- planning (`updateSQL`): changelog + referenced files are opened with read-locks
  while the plan is computed.
- execution (`update-count --count=1`): the same set is re-locked while the step
  runs.

If any lock cannot be acquired (file is being modified), `prepare:lb` fails.

---

## Error conditions

- Missing changelog or defaults file
- Search-path directory missing
- Liquibase exits non-zero
- Liquibase tries to access files outside mounted paths
- Engine storage/snapshot errors

---

## Event/logging behavior

Engine does not have a "verbose" mode. It always emits events:

- each line from Liquibase stdout -> task event
- Liquibase exit -> task status event

CLI decides how much to display, same as `prepare:psql`.

---

## Planning database target

Liquibase `updateSQL` needs a live database connection. For Postgres, a freshly
initialized cluster includes a default `postgres` database alongside `template0`
and `template1`, so sqlrs can connect to `postgres` on a brand-new instance for
planning. If `POSTGRES_DB` (or `POSTGRES_USER`) is set, the default database name
may differ; sqlrs should use the effective default database for the container.

---

## Filtering Liquibase arguments

sqlrs **filters** Liquibase arguments for safety and determinism:

- **Blocked commands**: anything outside the `update*` family (no rollback).
- **Blocked connection args**: any CLI flags that set `url/username/password`.
- **Blocked runtime args**: flags that change classpath or external driver behavior.
- **Search path**:
  - If `--searchPath` is passed, sqlrs mounts each path and rewrites it to
    `/sqlrs/mnt/pathN`.
  - In the future, we must also handle `searchPath` coming from environment or
    properties files.

All other arguments are passed through as-is.

---

## Examples (proposed)

```bash
sqlrs prepare:lb -- update \
  --changelog-file examples/liquibase/jhipster-sample-app/master.xml
```
