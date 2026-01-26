# CLI Architecture (Local and Remote)

This document describes how the `sqlrs` CLI resolves inputs and talks to the SQL Runner in local and shared deployments, including file vs URL handling and upload flows.

Note: references to `POST /runs` in this document describe the **runner API**
used in shared deployments (future). The current MVP local engine uses
`POST /v1/prepare-jobs` for prepare and `POST /v1/runs` for `sqlrs run`.

## 1. Goals

- Support the same CLI UX for local and remote targets.
- Allow inputs as local files or public URLs wherever a "file" is expected.
- Avoid large request bodies in `POST /runs`.
- Provide resumable, content-addressed uploads for remote execution.

## 2. Key Concepts

- **Target**: engine endpoint (local loopback or remote gateway).
- **Source**: project content (scripts, changelogs, configs).
- **Source ref**: either a local path, a public URL, or a server-side `source_id`.
- **Source storage**: service-side content store keyed by hashes and `source_id`.

## 3. Resolution Rules

For any CLI flag that expects a file or directory, the CLI accepts:

- **Local path** (file or directory).
- **Public URL** (HTTP/HTTPS).
- **Server-side source ID** (previously uploaded bundle).

Decision matrix:

| Target        | Input      | CLI action                                      |
| ------------- | ---------- | ----------------------------------------------- |
| Local engine  | Local path | pass path to engine                             |
| Local engine  | Public URL | pass URL to engine                              |
| Remote engine | Public URL | pass URL to engine                              |
| Remote engine | Local path | upload to source storage, then pass `source_id` |

## 4. Flows

### 4.1 Local target, local files

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant ENG as Local Engine

  CLI->>ENG: POST /runs (path:/repo/sql, entry=seed.sql)
  ENG->>ENG: read files from local FS
  ENG-->>CLI: stream status/results
```

### 4.2 Remote target, local files (upload then run)

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant GW as Gateway
  participant SS as Source Storage
  participant RUN as Runner

  CLI->>GW: POST /sources (create session)
  GW-->>CLI: source_id + chunk size
  loop for each chunk
    CLI->>GW: PUT /sources/{id}/chunks/{n}
  end
  CLI->>GW: POST /sources/{id}/finalize (manifest)
  GW->>SS: store bundle
  CLI->>GW: POST /runs (source_id)
  GW->>RUN: enqueue run
  RUN-->>CLI: stream status/results
```

### 4.3 Remote target, public URL

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant GW as Gateway
  participant RUN as Runner

  CLI->>GW: POST /runs (source_url=https://...)
  GW->>RUN: enqueue run
  RUN->>RUN: fetch URL into source cache
  RUN-->>CLI: stream status/results
```

### 4.4 Listing (sqlrs ls)

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant FS as "State Dir"
  participant ENG as Engine

  alt Local target
    CLI->>FS: read engine.json (endpoint + auth token)
    alt missing or stale
      CLI->>ENG: spawn local engine
      ENG-->>FS: write engine.json
      CLI->>FS: read engine.json
    end
  end

  CLI->>ENG: GET /v1/names (Authorization, Accept)
  ENG-->>CLI: JSON array or NDJSON
  CLI->>ENG: GET /v1/instances (Authorization, Accept)
  ENG-->>CLI: JSON array or NDJSON
  opt states requested
    CLI->>ENG: GET /v1/states (Authorization, Accept)
    ENG-->>CLI: JSON array or NDJSON
  end
  opt jobs requested
    CLI->>ENG: GET /v1/prepare-jobs (Authorization, Accept)
    ENG-->>CLI: JSON array or NDJSON
  end
  opt tasks requested
    CLI->>ENG: GET /v1/tasks?job=... (Authorization, Accept)
    ENG-->>CLI: JSON array or NDJSON
  end
  CLI->>CLI: render tables or JSON
```

Notes:

- Remote targets use the same list endpoints; the CLI supplies credentials from profile configuration.
- The CLI defaults to listing names and instances; states, jobs, and tasks are requested explicitly.
- Task listing can be filtered by job id (`--job`).

### 4.5 Removal (sqlrs rm)

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant ENG as Engine

  CLI->>ENG: GET /v1/instances?id_prefix=...
  ENG-->>CLI: instance list
  CLI->>ENG: GET /v1/states?id_prefix=...
  ENG-->>CLI: state list
  CLI->>ENG: GET /v1/prepare-jobs?job=...
  ENG-->>CLI: job list
  alt ambiguous or not found
    CLI->>CLI: error or noop
  else job resolved
    CLI->>ENG: DELETE /v1/prepare-jobs/{id}?force&dry_run
    ENG-->>CLI: DeleteResult
  else instance resolved
    CLI->>ENG: DELETE /v1/instances/{id}?force&dry_run
    ENG-->>CLI: DeleteResult
  else state resolved
    CLI->>ENG: DELETE /v1/states/{id}?recurse&force&dry_run
    ENG-->>CLI: DeleteResult
  end
  CLI->>CLI: render deletion tree
```

### 4.6 Run (sqlrs run)

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant ENG as Engine
  participant RT as Runtime

  alt Composite prepare present
    CLI->>ENG: start prepare job
    ENG-->>CLI: job_id + events_url + status_url
    CLI->>ENG: watch events (events_url)
    CLI->>ENG: GET /v1/prepare-jobs/{jobId} (on status events)
    ENG-->>CLI: terminal status + DSN
  else Explicit instance
    CLI->>ENG: resolve instance by id or name
    ENG-->>CLI: instance_id + DSN
  end

  CLI->>ENG: POST /v1/runs (kind, command or default, args)
  opt Missing instance container
    ENG-->>CLI: event "run: container missing - recreating"
    ENG-->>CLI: event "run: restoring runtime"
    ENG->>RT: start container with runtime_dir mount
    RT-->>ENG: container ready
    ENG-->>CLI: event "run: container started"
  end
  ENG->>RT: exec in instance container (inject DSN)
  RT-->>ENG: stdout/stderr/exit
  ENG-->>CLI: stream output + exit

  opt Composite invocation
    CLI->>ENG: delete instance
  end
```

Notes:

- `run:psql` passes DSN as a positional connection string; `run:pgbench` uses
  `-h/-p/-U/-d`.
- Commands run inside the instance container (same runtime as `prepare:psql`).
- If the instance container is missing and `runtime_dir` exists, the engine
  recreates the container and emits run events before execution.
- If `--instance` is provided together with a preceding `prepare`, the CLI fails
  with an explicit ambiguity error.
- Prepare monitoring is events-first; the CLI validates terminal status via
  `GET /v1/prepare-jobs/{jobId}` when status events arrive.

## 5. Upload Details (Remote)

- CLI chunks files, computes hashes, and uploads only missing chunks.
- A manifest maps file paths to chunk hashes; enables rsync-style delta.
- `source_id` is content-addressed and can be reused across runs.
- Large uploads are resumable; failed chunks can be retried without restarting.

## 6. Liquibase Presence

- If Liquibase is available, the CLI can request Liquibase-aware planning on the runner.
- If Liquibase is not available, CLI builds an explicit step plan (ordered script list) and passes it with the run request.
- The same upload/resolution rules apply in both modes.
