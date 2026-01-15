# ADR 0004: Local sqlrs prepare scope

Status: Accepted
Date: 2026-01-15

## Decision Record

- Timestamp: 2026-01-15T12:48:50+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What is the initial local scope for `sqlrs prepare`?
- Alternatives:
  - Support `prepare:psql` and `prepare:liquibase` from day one.
  - Support named instances and name binding flags (`--name`, `--reuse`, `--fresh`, `--rebind`).
  - Provide async `--watch/--no-watch` modes with `prepare_id`.
  - Restrict the base image to a Postgres-only runner.
- Decision: Limit local `sqlrs prepare` to `prepare:psql`, create ephemeral instances only, and run synchronously; Liquibase support, named instances, and async mode are deferred and tracked in `docs/architecture/cli-contract.md`. The engine relies on Docker and treats the image id as an opaque value, without a Postgres-only restriction.
- Rationale: Keeps the first implementation minimal and deterministic, aligns user docs with current behavior, and leaves room for later expansion without breaking the CLI contract.

## Context

- The local variant needs a basic, working `prepare` flow with minimal surface area.
- Several expansion paths (Liquibase, named instances, async mode) are valuable but add coordination and API surface.
- Docker is required in the local deployment; image selection should remain flexible.
