# ADR 0001: Engine listing API shape

Status: Accepted
Date: 2026-01-14

## Decision Record

- Timestamp: 2026-01-14T17:08:02+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What REST shape and auth/format rules should the engine listing API use for `sqlrs ls`?
- Alternatives:
  - Use non-REST list endpoints or `/by-name` paths.
  - Allow list filters on unique attributes instead of item endpoints.
  - Require JSON-only responses (no NDJSON) or pick output via query params.
  - Require auth on `/v1/health` as well.
- Decision: Use REST resource lists/items with `Accept`-driven JSON/NDJSON formats and auth required except `/v1/health`, and redirect name-based instance lookups.
- Rationale: Aligns with REST conventions, keeps list filters meaningful, supports streaming, and preserves simple health checks.

## Context

- `sqlrs ls` needs an engine API that follows REST resource patterns.
- List endpoints should support both JSON arrays and NDJSON streaming.
- Authorization is required for engine endpoints, except health checks.
- Filters on unique attributes are redundant; item endpoints should be used instead.
- Instance names are unique, but multiple names can alias the same instance.
- We want to avoid `/by-name` paths while still supporting name-based access.

## Decision

- Expose list endpoints:
  - `GET /v1/names`
  - `GET /v1/instances`
  - `GET /v1/states`
- Expose item endpoints:
  - `GET /v1/names/{name}`
  - `GET /v1/instances/{instanceId}`
  - `GET /v1/states/{stateId}`
- For list endpoints, `Accept` controls the format:
  - `application/json` returns arrays.
  - `application/x-ndjson` returns newline-delimited objects.
- Require `Authorization: Bearer <token>` for all endpoints except `/v1/health`.
- Filters are limited to non-unique selectors:
  - `/v1/names`: `instance`, `state`, `image`
  - `/v1/instances`: `state`, `image`
  - `/v1/states`: `kind`, `image`
- Instance item lookups support double addressing without a dedicated path:
  - If the path segment does not match the engine-defined instance id format,
    treat it as a name.
  - If it matches the id format, try id lookup first, then name lookup.
  - If resolved by name, respond with a `307 Temporary Redirect` to the canonical
    `/v1/{resource}/{resourceId}` URL.
- Disallow creation of names that collide with the instance id format to avoid
  ambiguity.

## Consequences

- Clients must follow redirects when resolving instances by name.
- List filtering is limited, encouraging item endpoints for unique identifiers.
- NDJSON enables efficient streaming for large lists without breaking JSON array clients.
