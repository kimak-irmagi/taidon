# ADR: Prepare job events range requests

- Timestamp: 2026-01-24
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)

## Question

How should clients resume reading `/v1/prepare-jobs/{jobId}/events` after a
disconnect without re-downloading all events?

## Alternatives considered

1) Query parameters (e.g. `?from_event=10`)
2) HTTP range requests using an events-based unit (e.g. `Range: events=10-`)
3) No resume support (always restart from the beginning)

## Decision

Use HTTP range requests with an events-based unit.

- Client sends `Range: events=10-` or `Range: events=10-24`.
- Server may respond with `206 Partial Content` and
  `Content-Range: events 10-24/100` when supported.
- If range is unsupported, server returns `200 OK` with the full stream.

## Rationale

The range mechanism is explicit, standard-compatible, and allows the server to
opt in without breaking compatibility. It also keeps the URL stable and
centralizes resumption semantics in headers rather than duplicating query
parameter logic.
