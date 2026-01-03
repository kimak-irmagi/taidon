# CLI Architecture (Local and Remote)

This document describes how the `sqlrs` CLI resolves inputs and talks to the SQL Runner in local and shared deployments, including file vs URL handling and upload flows.

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

| Target | Input | CLI action |
|---|---|---|
| Local engine | Local path | pass path to engine |
| Local engine | Public URL | pass URL to engine |
| Remote engine | Public URL | pass URL to engine |
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

## 5. Upload Details (Remote)

- CLI chunks files, computes hashes, and uploads only missing chunks.
- A manifest maps file paths to chunk hashes; enables rsync-style delta.
- `source_id` is content-addressed and can be reused across runs.
- Large uploads are resumable; failed chunks can be retried without restarting.

## 6. Liquibase Presence

- If Liquibase is available, the CLI can request Liquibase-aware planning on the runner.
- If Liquibase is not available, CLI builds an explicit step plan (ordered script list) and passes it with the run request.
- The same upload/resolution rules apply in both modes.
