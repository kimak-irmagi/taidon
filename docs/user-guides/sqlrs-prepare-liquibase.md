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
- Supporting non-container Liquibase installations.
- Custom Java extensions / classpath injection beyond standard Liquibase image contents.

---

## CLI Syntax (proposed)

```text
sqlrs prepare:lb [--image <db-image-id>]
                             [--liquibase-image <image-id>]
                             -- <liquibase-args...>
```

### Flags

- `--image <db-image-id>` (optional): overrides the DB base image (same as `prepare:psql`).
- `--liquibase-image <image-id>` (optional): overrides Liquibase CLI image.
- `liquibase-args...` (required): passed to Liquibase CLI after `--`.

### Config fallback

`--liquibase-image` overrides config. Otherwise:

```yaml
liquibase:
  image: liquibase/liquibase:latest
```

---

## Local Execution Model

1. Engine creates (or reuses) a base state for the DB image (same as `prepare:psql`).
2. Engine starts a **Liquibase container** that connects to the prepared DB instance.
3. User provides **Liquibase command line** after `--` (for example `update`).
4. Engine executes **plan** via `updateSQL`, inspects the resulting changesets and
   builds a fine-grained plan.
5. Engine executes the plan as a sequence of `updateOne` steps, snapshotting after
   each changeset.
6. The prepared state is cached and a new instance is created.

### Planning vs execution tasks

- **Plan task**: run `liquibase updateSQL` and **parse its output** to compute the
  ordered list of changesets and the SQL for each changeset (no DB changes).
- **Prepare tasks**: execute changesets **one by one** with `liquibase updateOne`,
  emitting events per task (stdout lines -> events, exit -> status event).

### Connection parameters

Connection arguments are **always** supplied by the engine and override defaults file values.
User-provided `--url`, `--username`, `--password`, `--classpath`, etc. are rejected.

---

## File resolution behavior (Liquibase)

Liquibase CLI resolves files relative to its **current working directory**, plus
classpath paths and a configured search path. When `--search-path` is set, it
overrides the default search locations. By default, Liquibase looks for a
`liquibase.properties` file in the directory where it is run.

**Proposed behavior in sqlrs**:

- The Liquibase container **working directory** is the CLI working directory.
- sqlrs does **not** attempt to resolve includes itself. Liquibase handles it.

This keeps mounts minimal while delegating include resolution to Liquibase.

---

## Mounting strategy (local)

When running Liquibase in a container, sqlrs mounts local paths referenced by
Liquibase arguments into the container. Each path is mounted read-only and rewritten
to `/sqlrs/mnt/pathN`.

If no local paths are found, sqlrs mounts the **current working directory** as a
single mount root.

---

## Deterministic fingerprint (state id)

State identification is based on the **ordered list of Liquibase changesets**
returned by planning, plus the previous state:

- `prepare kind` = `lb`
- `prev_state_id`
- ordered list of changesets as reported by Liquibase:
  - `id`, `author`, `path`
  - `sql_hash` (hash of SQL emitted for that changeset by `updateSQL`, parsed
    from the `updateSQL` output)

If two different argument sets produce the same ordered changesets (including
the per-changeset SQL hashes), sqlrs reuses the cached state for that chain.

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
