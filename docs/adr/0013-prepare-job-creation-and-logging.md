# ADR 0013: prepare job creation semantics and engine logging

Status: Accepted
Date: 2026-01-19

## Decision Record 1: prepare job creation response code

- Timestamp: 2026-01-19T18:20:55.8194278+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: When should the engine respond to `POST /v1/prepare-jobs`, and which status code should it use?
- Alternatives:
  - Keep `202 Accepted` and rely on client polling/backoff.
  - Add client-side delays/retries before the first GET.
  - Persist the job record first, return `201 Created`, then continue async execution.
- Decision: Persist the job record before responding and return `201 Created`.
- Rationale: Ensures immediate GETs see the job while keeping execution async and reducing client polling complexity.

## Decision Record 2: engine execution logging scope

- Timestamp: 2026-01-19T18:20:55.8194278+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What should the engine log to diagnose prepare job ordering and execution issues?
- Alternatives:
  - Log only errors.
  - Log HTTP requests only.
  - Log HTTP requests and prepare lifecycle steps (job creation, planning, task status updates, completion).
- Decision: Log HTTP requests and key prepare lifecycle steps.
- Rationale: Provides actionable traces for ordering issues without requiring client changes.
