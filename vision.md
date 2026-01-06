# Taidon - Vision

## 1. Introduction

Taidon is a new model for working with databases, built around the idea of reproducible state. We treat SQL not as a stream of disconnected operations on a "live" database, but as a deterministic process of building state, where any sequence of queries uniquely determines the final structure and content of the database.

This approach removes a fundamental limitation of traditional DBMSs: dependence on the current state that changes with each query. It opens a path to more predictable learning, testing, experimentation, and automation.

## 2. The problem we solve

Working with databases today often resembles working with a program component whose state changes irreversibly:

- A student's mistake can damage a training database.
- Parallel learning requires many separate deployments.
- Testing depends on the state left by previous actions.
- Preparing data for experiments and checks is time-consuming and resource-heavy.
- In CI/CD, replaying migrations and tests is slow and expensive.

Built-in snapshot mechanisms in existing DBMSs were designed for resilience and transactional isolation, not for mass work with branched state histories. They are heavy, scale poorly, and require significant hardware resources.

## 3. Core idea

We treat database state as the result of executing a sequence of queries. The system manages this sequence so that states are:

- reproducible,
- cacheable,
- shared across users,
- available on demand without replaying all steps.

Key elements:

- each query history has a unique identifier,
- the system looks up the corresponding state in cache,
- if found, it is restored instantly,
- if not, the system replays the history, produces a new state, and stores it,
- branched histories are supported naturally (shared prefixes lead to shared states).

This turns SQL into an analogue of a deterministic build system, but for data.

## 4. How Taidon works

1. The user creates a project of SQL scripts organized in folders: e.g., init/, seed/, test/, bench/.
2. Taidon interprets the project as a state graph where each query extends the history.
3. When a script runs, the system checks if the state corresponding to previous steps already exists.
4. If it exists, it is restored almost instantly.
5. If not, the system replays the entire history, produces a new state, and stores it.
6. Retention policy optimizes cache by rebuild cost and usage frequency.
7. Internals (containers, copy-on-write filesystems, snapshot mechanisms) are hidden from the user.

The user works with the project like with any codebase, without manually managing a DB server.

## 5. What the user gets

### Instant start

You can begin immediately, without deploying a DB or preparing the environment.

### Reproducibility

Any query can be rerun on an exact copy of the database; the state is computed from the same sequence of queries.

### Branching scenarios

You can create parallel variants of database evolution: different migrations, test variants, alternative experiment queries.

### Scalable learning

Hundreds of users can operate on a shared logical database while remaining independent and without fully duplicating data.

### Precise SQL diagnostics

The Taidon editor provides:

- correct error localization,
- accurate PostgreSQL messages,
- interactive hints.

### Support for large projects

Organized SQL scripts and reproducible state make Taidon useful for migrations, CI/CD, and complex test scenarios.

## 6. What makes Taidon unique

### We do not clone the database — we compute state

State is formed by re-executing queries, not by copying files.

### We hide infrastructure

The user works with SQL, not with containers, storage, or filesystems.

### We use real PostgreSQL

This ensures compatibility and accuracy of results.

### We remove duplication

Different users and branches can share common parts of history.

### We do not limit professional usage

You can run complex queries, compare PostgreSQL versions, and study performance.

## 7. Development strategy

### Stage 1 - MVP

- Base mechanism for replaying query chains.
- PostgreSQL support.
- State cache development.
- Interactive SQL editor.
- Platform interface (CLI and basic UI).

### Stage 2 - Capability expansion

- State access via API.
- Additional pricing tiers.
- Support for alternative SQL engines.
- Improved cache and storage formats.
- Better change handling and optimization.

### Stage 3 - Generalization

- Support for other DBMS classes.
- Integrations with education platforms and CI/CD.
- IDE and tooling support (VS Code, DBeaver).

## 8. Long-term goal

Create a universal layer for reproducible work with data that:

- makes database work predictable,
- lowers the cost of environment preparation,
- scales learning,
- ensures experimental accuracy,
- simplifies development and delivery of database-heavy products.

The ideal: Taidon becomes for data what Git became for source code.
