# Prepare Job Events Component Structure

This document captures the internal structure needed to support events-first
monitoring across CLI and engine.

## CLI (frontend/cli-go)

Modules:

- `internal/client`
  - HTTP request helpers and typed JSON decoding.
  - Add an events stream reader that:
    - Accepts `events_url`.
    - Supports `Range: events=...` on reconnect.
    - Exposes a channel or iterator of `PrepareJobEvent`.

- `internal/cli`
  - `waitForPrepare` becomes events-first:
    - Opens stream, parses NDJSON.
    - On `status` events, re-fetches job status.
    - Stops on confirmed `succeeded`/`failed`.
    - Errors if stream ends without definitive status.
  - Rendering:
    - Status line updates on new events.
    - Spinner animation for repetitive events.
    - Includes event `message` when present.
    - In verbose mode, prints each event on a new line.

- `internal/app`
  - No structural change beyond using updated `RunPrepare` behavior.

Data ownership:

- Events stream state (last event index, spinner state) is in-memory only.
- Final job result is obtained from `GET /v1/prepare-jobs/{jobId}`.

## Engine (backend/local-engine-go)

Modules:

- `internal/httpapi`
  - `/v1/prepare-jobs/{jobId}/events` supports:
    - Range parsing for `Range: events=...` (optional).
    - `206 Partial Content` with `Content-Range: events`.
    - `Accept-Ranges: events` when supported.

- `internal/prepare`
  - Event bus and storage already exist.
  - Optional range support reads from queue by event index.
  - Emits log events for runtime/DBMS operations and heartbeat task events
    (~500ms) when no new events arrive while a task is running.

Data ownership:

- Events are stored in the prepare queue (SQLite) and streamed from there.
- Streaming remains stateless beyond the requested `events` range.

## Deployment Units

- CLI: reads events, validates final status.
- Engine: produces and stores events, streams them to clients.
