# sql-runner

Service for deterministic execution of SQL chains. Computes head/tail, looks up a ready state snapshot in `snapshot-cache`, boots or reuses a instance via `env-manager`, executes head, and returns the result with layer metadata.

## Status

MVP is under design; this README captures target behavior and interfaces ahead of code.

## Role in the platform

- deterministically replays SQL chains as a function of project content;
- computes tail hash and requests the matching layer from `snapshot-cache`;
- boots/restarts a PostgreSQL instance via `env-manager` and executes head;
- returns query results, compile/runtime errors, and layer metadata;
- emits audit and telemetry for monitoring and investigations.

## Key execution flow

1. `gateway` proxies a request from the frontend with the SQL project tree and head pointer.
2. Planner builds the chain, splits it into tail/head, and computes tail hash.
3. `snapshot-cache` client looks up an existing layer; on miss it requests tail replay and layer creation.
4. `env-manager` brings up a instance from the found/created layer or from a base image.
5. Executor connects to instance PostgreSQL, runs head, captures errors/results.
6. If needed, a new layer is published to `snapshot-cache`.
7. Response goes to `gateway` with layer data, logs, and metrics.

## Module structure (MVP)

- API handler - HTTP/GRPC entry, integrated through `gateway`.
- Planner - tree parsing, tail/head split, tail hash calculation.
- Snapshot-cache client - lookup/publish layers, account for engine version and settings.
- Env binding - interaction with `env-manager`, instance TTL/limit control.
- Executor - connects to PostgreSQL and runs head with user parameters.
- Observability hooks - metrics, audit, traces, correlation by `request_id`.

## SQL project and hashing

- Project is a tree of SQL scripts (`init/`, `seed/`, `test/`, `bench/`, etc.).
- Head is the specific file executed to return user results.
- Tail is all preceding files/steps; its hash is used as the cache key.
- Tail is hashed by content, paths, engine/extension version, and environment settings.
- Branching is supported naturally: common prefix yields shared layers.

## Job model

- Each chain runs as a job. Jobs are created, executed asynchronously, support progress, cancellation, and GC.
- Job types:
  - **short** - created and executed in one request (fire-and-forget + quick polling).
  - **long** - built in multiple parts (append to an existing job) to avoid large request bodies and to stream progress as the chain is built.
- Progress: status (`pending` -> `preparing` -> `running` -> `publishing` -> `completed`/`failed`/`canceled`), stages (parsing, snapshot lookup, instance boot, head execution), intermediate logs.
- Job TTL: `job_ttl` set at creation; advanced TTL/extension available in premium plans. GC periodically removes expired jobs and artifacts.

## API (draft)

- `POST /v1/jobs` - create job.
  - Input: `request_id`, `user_id`, `project_tree` (path -> content/etag), `head_path`, `variables`, `snapshot_ttl`, `job_ttl`, `timeout_ms`, `telemetry_context`.
  - Used for short chains: send the whole project and get `job_id` + immediate start response.
- `POST /v1/jobs/{job_id}/append` - append a part of the project/chain to an existing job (long scenarios).
- `POST /v1/jobs/{job_id}/run` - start execution if the job was assembled in parts.
- `GET /v1/jobs/{job_id}` - status, progress, current stage, tail_hash, links to logs/results.
- `GET /v1/jobs/{job_id}/logs` - incremental logs (polling) or SSE/WS (if added).
- `POST /v1/jobs/{job_id}/cancel` - cancel execution and clean up instance.
- Webhooks (optional): notifications on `completed`/`failed`/`canceled` for external integrations.
- Success response:
  - `status` (`completed`), `rows` or error/cancel description;
  - `snapshot` (tail_hash, layer_id, hit/miss);
  - `progress` (final stage, duration), `logs` (last chunk) or link;
  - `metrics` (latency, cache_hit, retries).

## Integrations

- `snapshot-cache` - lookup/publish layers by `tail_hash`.
- `env-manager` - instance create/restart, network policy (PostgreSQL accessible only to `sql-runner`).
- `audit-log` - user actions and errors.
- `telemetry/exporter` - Prometheus metrics and traces.

## Observability and safety

- Metrics: cold/warm instance start time, head duration, cache hit rate, connection/SQL errors, job progress, cancel/timeout rates, GC load.
- Logs: structured, no user data; correlated via `request_id` and `job_id`.
- Audit: who/what/when with result/cancel status.
- Network boundaries: access to PostgreSQL instances is limited to `sql-runner`; external calls go through `gateway` only.

## Local development (sketch)

- Service code not added yet; launch details will be provided when it appears.
- Plan: use `infra/local-dev/` for docker-compose/minikube, prepare env profiles and variables (`POSTGRES_DSN`, `SNAPSHOT_CACHE_URL`, `ENV_MANAGER_URL`, etc.).
- Commands for build/run/tests/linters will be added once code exists.

## Backlog README

- Clarify `project_tree` format, tail/head rules, and append contract.
- Add error schemas and response codes.
- Describe layer dedup/GC policy, instance TTL, and job TTL (including premium plan rules).
- Update local run section after code and infra manifests land.
