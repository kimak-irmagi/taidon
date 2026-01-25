# Prepare Job Events Flow

This document describes the events-first flow for monitoring `prepare` jobs,
with minimal polling and explicit reconnection behavior.

## Interaction Flow

1) CLI submits job
   - `POST /v1/prepare-jobs`
   - Response includes `job_id`, `status_url`, `events_url`

2) CLI opens events stream
   - `GET {events_url}`
   - Response is NDJSON (`application/x-ndjson`)
   - CLI reads until stream completion (see completion rules below).

3) CLI renders progress
   - Each event updates the displayed status line.
   - Repetitive events may be animated via a spinner without new lines.
   - Log events include messages from runtime/DBMS operations (Docker/Postgres).
   - In verbose mode, each event is printed on a new line.

4) Status validation on status events
   - On any `status` event (queued/running/succeeded/failed), the CLI calls:
     `GET /v1/prepare-jobs/{jobId}`.
   - If the status endpoint returns `succeeded` or `failed`, the CLI stops.

5) Stream completion
   - If HTTP status is 4xx, the CLI fails.
   - If `Content-Length` is present, the stream completes when all declared
     bytes are read.
   - If the stream completes without a definitive job status, the CLI fails.

6) Reconnect behavior (optional)
   - On disconnect, the CLI resumes using `Range: events=...`.
   - If the server ignores the range and returns 200, the CLI restarts from
     the beginning.

7) Heartbeat behavior
   - While a task is running, the engine repeats the last task event with a
     fresh timestamp when no new events arrive for ~500ms.

## Completion Rules

- Success: job status confirmed as `succeeded`.
- Failure: job status confirmed as `failed`, or stream ends without a
  definitive status, or any 4xx response.

## Sequence Diagram (informal)

```text
CLI -> Engine: POST /v1/prepare-jobs
Engine -> CLI: 201 { job_id, events_url, status_url }
CLI -> Engine: GET events_url
Engine -> CLI: NDJSON stream of PrepareJobEvent
CLI -> Engine: GET /v1/prepare-jobs/{jobId} (on status events)
Engine -> CLI: PrepareJobStatus (succeeded|failed)
```
