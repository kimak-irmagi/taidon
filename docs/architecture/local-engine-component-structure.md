# Local Engine Component Structure

This document defines the internal component layout of the local sqlrs engine.

## 1. Goals

- Make module boundaries explicit before implementation.
- Separate HTTP, auth, domain logic, and persistence.
- Keep names/instances/states in persistent storage, not in-memory only.

## 2. Packages and responsibilities

- `cmd/sqlrs-engine`
  - Parse flags and build dependencies.
  - Start the HTTP server.
  - Resolve `SQLRS_STATE_STORE` and validate the WSL systemd mount when configured.
- `internal/httpapi`
  - HTTP routing and handlers.
  - JSON/NDJSON encoding.
  - Uses auth + registry + prepare + store + config interfaces.
  - Exposes job/task list endpoints and job deletion.
- `internal/config`
  - Loads defaults + persisted overrides on startup.
  - Provides get/set/rm with path resolution and schema validation.
  - Persists JSON atomically next to the state store.
  - Exposes the config schema to HTTP handlers.
- `internal/prepare`
  - Prepare job coordination (plan, cache lookup, execute, snapshot).
  - Handles `plan_only` jobs and task list output.
  - Persists job/task state and event history via the queue store.
  - Supports job listing and deletion with force/dry-run.
- `internal/prepare/queue`
  - SQLite-backed queue store for jobs, tasks, and events.
  - Supports restart recovery by reloading queued/running work.
  - Trims completed prepare jobs beyond the per-signature retention limit from config (`orchestrator.jobs.maxIdentical`).
  - Removes `state-store/jobs/<job_id>` when jobs are deleted.
- `internal/executor`
  - Runs job tasks sequentially and emits task/job events.
  - Invokes snapshot manager and DBMS connector around snapshots.
- `internal/runtime`
  - Docker runtime adapter (CLI-based in MVP).
  - Starts/stops containers; sets `PGDATA` and trust auth for Postgres images.
  - Keeps warm containers after prepare until run orchestration decides to stop.
- `internal/run`
  - Resolves target instance (id/name).
  - Prepares runtime exec context and validates run kind rules.
  - Executes commands inside instance containers and streams output.
  - Recreates missing instance containers from `runtime_dir` and updates
    `runtime_id` before executing run commands.
- `internal/snapshot`
  - Snapshot manager interface and backend selection.
  - OverlayFS or btrfs snapshots in the MVP, copy fallback.
- `internal/dbms`
  - DBMS-specific hooks for snapshot preparation and resume.
  - Postgres implementation uses `pg_ctl` for fast shutdown/restart without stopping the container.
- `internal/deletion`
  - Builds deletion trees for instances and states.
  - Enforces recurse/force rules and executes removals.
- `internal/conntrack`
  - Tracks active connections per instance via DB introspection.
- `internal/auth`
  - Bearer token verification.
  - Exempt `/v1/health`.
- `internal/registry`
  - Domain rules for lookups and redirect decisions.
  - ID vs name resolution for instances.
  - Reject names that match the instance id format.
- `internal/id`
  - Parsing/validation of id formats.
- `internal/store`
  - Interfaces for names, instances, states.
  - Filter types for list calls.
- `internal/store/sqlite`
  - SQLite-backed implementation.
  - Database file under `<StateDir>`.
  - Implements `internal/store` interfaces.
- `internal/stream`
  - NDJSON writer helpers.

## 3. Key types and interfaces

- `prepare.Manager`
  - Submits jobs and exposes status/events.
  - Handles `plan_only` by returning task lists.
- `run.Manager`
  - Validates run requests and executes commands against instances.
  - Streams stdout/stderr/exit back to HTTP.
  - Emits recovery events when recreating a missing instance container.
- `queue.Store`
  - Persists jobs, tasks, and events; supports recovery queries.
- `prepare.JobEntry`, `prepare.TaskEntry`
  - List views for jobs and task queue entries.
- `prepare.Request`, `prepare.Status`
  - Job request and status payloads (includes `tasks` for plan-only).
- `prepare.PlanTask`, `prepare.TaskInput`
  - Task descriptions and input references for planning/execution.
- `config.Manager`
  - Get/set/remove config values and return schema.
- `config.Value`, `config.Schema`
  - JSON values and schema representation.
- `store.Store`
  - Interface for names/instances/states persistence.
- `deletion.Manager`
  - Builds delete trees and executes deletions.

## 4. Data ownership

- Persistent data (names/instances/states) lives in SQLite under `<StateDir>`.
- Jobs, tasks, and job events live in SQLite under `<StateDir>`.
- In-memory structures are caches or request-scoped only.
- State store data lives under `<StateDir>/state-store` unless `SQLRS_STATE_STORE` overrides it.
- Server config is stored in `<StateDir>/state-store/config.json` and mirrored in memory.

## 5. Dependency diagram

```mermaid
flowchart TD
  CMD["cmd/sqlrs-engine"]
  HTTP["internal/httpapi"]
  PREP["internal/prepare"]
  QUEUE["internal/prepare/queue"]
  EXEC["internal/executor"]
  RT["internal/runtime"]
  SNAP["internal/snapshot"]
  DBMS["internal/dbms"]
  DEL["internal/deletion"]
  CONN["internal/conntrack"]
  AUTH["internal/auth"]
  REG["internal/registry"]
  ID["internal/id"]
  STORE["internal/store (interfaces)"]
  SQLITE["internal/store/sqlite"]
  STREAM["internal/stream (ndjson)"]
  CFG["internal/config"]

  CMD --> HTTP
  CMD --> SQLITE
  CMD --> CFG
  HTTP --> AUTH
  HTTP --> PREP
  HTTP --> RUN
  HTTP --> DEL
  HTTP --> REG
  HTTP --> STREAM
  HTTP --> CFG
  PREP --> QUEUE
  PREP --> EXEC
  PREP --> CFG
  EXEC --> RT
  EXEC --> SNAP
  EXEC --> DBMS
  DEL --> STORE
  DEL --> CONN
  RUN --> RT
  RUN --> REG
  PREP --> STORE
  REG --> ID
  REG --> STORE
  SQLITE --> STORE
```
