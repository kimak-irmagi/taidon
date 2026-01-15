# Standard Workflow: Query Experiments (Baseline → Liquibase → sqlrs)

This document describes a **minimal-interference** workflow for running a complex `SELECT` across multiple database "versions" (schema/index variants, data volumes) and collecting results, plans, and basic metrics.

The key idea: keep using the same familiar tools (`psql`, `pgbench`, scripts). The only thing that changes with `sqlrs` is **how fast and repeatably you get the target database state**.

Note: Liquibase examples below are aspirational; the current local prepare supports only `prepare:psql`.

---

## 1. User Goal

- Run the same query against multiple DB states, e.g.:
  - `0-1-3`: schema v1 + small seed + upgrade
  - `0-2-3`: schema v1 + large seed + upgrade
  - plus index variants or configuration variants
- Collect one or more of:
  - query result (optional)
  - query plan (EXPLAIN)
  - execution stats (EXPLAIN ANALYZE, buffers)
  - benchmark timings (repeated runs)

---

## 2. Baseline A: No Liquibase (manual scripts)

### 2.1 Prepare two DB instances (two "versions")

Typical practice: run multiple Postgres containers on different ports.

```bash
# instance A: small seed
docker run -d --name pg_small -e POSTGRES_PASSWORD=postgres -p 5433:5432 postgres:17

# instance B: large seed
docker run -d --name pg_large -e POSTGRES_PASSWORD=postgres -p 5434:5432 postgres:17
```

Then apply scripts:

```bash
# schema
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f schema.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f schema.sql

# seed variants
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f seed_small.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f seed_large.sql

# optional upgrade
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f upgrade_v2.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f upgrade_v2.sql

# ensure planner statistics are comparable
psql "postgresql://postgres:postgres@localhost:5433/postgres" -c "ANALYZE;"
psql "postgresql://postgres:postgres@localhost:5434/postgres" -c "ANALYZE;"
```

### 2.2 Run the query and capture plans

```bash
QFILE=query.sql

# plan only
psql "postgresql://postgres:postgres@localhost:5433/postgres" \
  -v ON_ERROR_STOP=1 -X -f - <<'SQL'
\timing on
EXPLAIN (VERBOSE, COSTS, BUFFERS, SETTINGS, FORMAT TEXT)
\i query.sql
SQL

# plan + runtime stats
psql "postgresql://postgres:postgres@localhost:5433/postgres" \
  -v ON_ERROR_STOP=1 -X -f - <<'SQL'
\timing on
EXPLAIN (ANALYZE, VERBOSE, BUFFERS, SETTINGS, FORMAT TEXT)
\i query.sql
SQL
```

### 2.3 Benchmark (repeated runs)

Two common patterns:

1. simple loop with warmup:

   ```bash
   # warmup
   for i in 1 2 3; do psql "$DSN" -c "\i query.sql" >/dev/null; done
 
   # measure
   for i in 1 2 3 4 5; do time psql "$DSN" -c "\i query.sql" >/dev/null; done
   ```

2. pgbench custom script:

   ```bash
   pgbench "$DSN" -f query.sql -T 30 -c 1 -j 1
   ```

### 2.4 Common best practices (manual)

- Make states reproducible with scripts + deterministic seeds.
- Run `ANALYZE` after data load; otherwise plans can differ for irrelevant reasons.
- Decide whether you compare **warm-cache** vs **cold-cache**.
  - warm-cache: warm up then measure
  - cold-cache: restart the DB instance between runs
- Fix important settings if comparing performance (e.g. `work_mem`, `jit`).

---

## 3. Baseline B: With Liquibase (standard team practice)

### 3.1 Represent variants with contexts/labels

Typical:

- schema + upgrades: always apply
- seed variants: selected by `context-filter` or `label-filter`

Run two states by pointing to two separate DB instances:

```bash
# small
liquibase \
  --url="jdbc:postgresql://localhost:5433/postgres" \
  --username=postgres --password=postgres \
  --changelog-file=db.changelog.xml \
  --context-filter=small \
  update

# large
liquibase \
  --url="jdbc:postgresql://localhost:5434/postgres" \
  --username=postgres --password=postgres \
  --changelog-file=db.changelog.xml \
  --context-filter=large \
  update
```

Then do the same execution step as in Section 2.2 / 2.3 using `psql` and `pgbench`.

### 3.2 Common best practices (Liquibase)

- Keep seed choices behind contexts/labels.
- Split huge seed into predictable units (or generate deterministically).
- Keep a consistent Postgres version and settings when comparing performance.

---

## 4. With sqlrs (minimal interference, unified prepare + run)

### 4.1 Core idea

With `sqlrs`, the database **connection URL is derived from a logical state specification**, not from a pre-existing instance.

A _state specification_ (StateSpec) combines:

- base engine and version (e.g. `postgres:17`)
- preparation recipe (e.g. Liquibase changelog + filters)

`sqlrs` guarantees that this logical state exists (via cache or materialisation) and then provides a **standard DSN / connection URL** to the user tool.

The presence or absence of cache affects **only startup time**, never semantics.

---

### 4.2 Unified prepare + run workflow

Instead of explicitly creating instances or managing global tags, the recommended workflow is a **single command** that:

1. describes how to prepare the database state;
2. immediately runs a user tool against that state.

Example: benchmark with a small seed

```bash
sqlrs run \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small \
  -- \
  pgbench -f query.sql -T 30 -c 1 -j 1
```

Example: EXPLAIN ANALYZE with a large seed

```bash
sqlrs run \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=large \
  -- \
  psql -v ON_ERROR_STOP=1 -X \
    -c "EXPLAIN (ANALYZE, VERBOSE, BUFFERS, SETTINGS) $(cat query.sql)"
```

Semantics:

- `--prepare liquibase ...` defines the logical database history (StateSpec).

- sqlrs resolves this StateSpec to a concrete immutable state (cache hit or build).

- An ephemeral instance is created from that state.

- The wrapped command receives a normal database connection via env/DSN.

### 4.3 Alternative: explicit DSN or environment export

For tools that cannot easily be wrapped, sqlrs can expose connection details directly:

```bash
sqlrs url \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small
```

or:

```bash
eval "$(sqlrs env \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small)"

pgbench -f query.sql -T 30 -c 1 -j 1
```

This preserves compatibility with existing scripts and interactive workflows.

### 4.4 Best practices (sqlrs)

- Treat preparation parameters as the **true identity** of a database state.
- Avoid global human-readable tags unless explicitly needed; prefer inline StateSpec.

- Use ephemeral instances for measurements to avoid cross-run contamination.
- Keep the same DB engine/version and relevant settings across comparisons.
- For large seeds, rely on cache pinning rather than manual tagging.

## 5. Checklist for a Good Experiment

- [ ] Same engine + version
- [ ] Same settings that influence planner/runtime (document them)
- [ ] `ANALYZE` performed after data load
- [ ] Warmup policy defined (warm vs cold)
- [ ] Collected artifacts saved (stdout, explain, timings)

---

## 6. Notes

- Liquibase is about _preparing_ database states.
- `sqlrs` is about _materialising and reusing_ many database states quickly.
- Query analysis tools remain the same in MVP.
