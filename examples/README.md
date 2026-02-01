# SQL Examples

This directory contains **ready-to-run SQL datasets** and **Liquibase changelog examples**
used as examples and test baselines. The files themselves are **fetched from upstream open-source projects on demand**
and are **not fully vendored** in the repository.

## What’s included

Currently supported example datasets:

- **Chinook (PostgreSQL)**  
  A small, well-known sample database for SQL experiments.

- **Sakila (PostgreSQL port)**  
  PostgreSQL-adapted version of the classic Sakila sample database.

- **Flights / Airlines (PostgresPro demo DB)**  
  Realistic airline booking dataset from Postgres Professional.

Liquibase examples (open-source projects):

- **JHipster Sample App (Liquibase XML)**  
  Multi-file changelog set with includes and constraints.

- **Liquibase GitHub Action Example (formatted SQL)**  
  Single-file changelog in formatted SQL style.

All sources are listed with licenses and upstream links in  
[`scripts/external/NOTICE.md`](scripts/external/NOTICE.md).

---

## Directory layout

After fetching, the layout looks like this:

```text
examples/
  chinook/
    Chinook_PostgreSql.sql
  sakila/
    0-postgres-sakila-setup.sql
    1-postgres-sakila-schema.sql
    2-postgres-sakila-insert-data.sql
    3-postgres-sakila-user.sql
  flights/
    demo-small-en-20170815.sql
  liquibase/
    jhipster-sample-app/
      master.xml
      00000000000000_initial_schema.xml
      20150805124838_added_entity_BankAccount.xml
      20150805124936_added_entity_Label.xml
      20150805125054_added_entity_Operation.xml
      20150805124838_added_entity_constraints_BankAccount.xml
      20150805125054_added_entity_constraints_Operation.xml
    liquibase-github-action-example/
      samplechangelog.h2.sql

The listed files under examples/ are generated artifacts and may be overwritten by re-fetching.

## Fetching the example files

The SQL and Liquibase files are downloaded using the fetch script:

```bash
pnpm install
pnpm fetch:sql
```

This will:

1. Download files from upstream sources
2. Verify integrity using sha256
3. Place results directly into `./examples/...`

### First-time locking of checksums

If a source has no checksum yet:

```bash
pnpm fetch:sql --write-sha
```

This computes and writes sha256 values into `scripts/external/manifest.yaml`.

### Reproducible / CI mode

For CI and reproducible runs:

```bash
pnpm fetch:sql --lock
```

---

## Running examples (plain Docker)

### Chinook

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/chinook:/sql" \
  postgres:17 \
  psql -U postgres -f /sql/Chinook_PostgreSql.sql
```

### Sakila

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/sakila:/sql" \
  postgres:17 \
  bash -c "
    psql -U postgres -f /sql/0-postgres-sakila-setup.sql &&
    psql -U postgres -f /sql/1-postgres-sakila-schema.sql &&
    psql -U postgres -f /sql/2-postgres-sakila-insert-data.sql &&
    psql -U postgres -f /sql/3-postgres-sakila-user.sql
  "
```

### Flights (PostgresPro demo DB)

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/flights:/sql" \
  postgres:17 \
  psql -U postgres -f /sql/demo-small-en-20170815.sql
```

## Running via `sqlrs` (recommended)

These examples are designed to be used as `--prepare` inputs for `sqlrs`.

Example:

```bash
sqlrs \
  --from postgres:17 \
  --workspace ./sqlrs-work \
  --prepare examples/flights/demo-small-en-20170815.sql \
  -run -- psql -f examples/flights/queries.sql
```

## Notes on updates and reproducibility

- Upstream sources may evolve — checksums prevent silent changes.
- `examples/` is treated as derived data, not hand-maintained code.
- To update an upstream dataset:
  1. Update URL / revision in `manifest.yaml`
  2. Run `pnpm fetch:sql --write-sha`
  3. Commit the updated manifest

## Licenses

Each dataset is distributed under its original license.
See [`scripts/external/NOTICE.md`](scripts/external/NOTICE.md) for details and attribution.
