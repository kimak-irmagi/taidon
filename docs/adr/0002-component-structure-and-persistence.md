# ADR 0002: Component structure and persistence boundaries

Status: Accepted
Date: 2026-01-14

## Decision Record

- Timestamp: 2026-01-14T17:08:02+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should component boundaries and persistence be defined for the CLI and local engine?
- Alternatives:
  - Keep engine logic in a single package with in-memory collections.
  - Document only deployment tiers (CLI/engine) without internal component structure.
  - Persist registry data outside the engine (or not at all).
- Decision: Define component-structure docs for CLI and local engine, introduce `internal/store` interfaces with a SQLite implementation, and persist names/instances/states in SQLite under `<StateDir>`.
- Rationale: Clear boundaries reduce coupling, SQLite provides durable registry data, and documented structure guides implementation.

## Context

- The project needs explicit internal component structure per deployment unit.
- The engine must not keep names/instances/states only in memory.
- We need clear interface vs implementation boundaries to avoid tight coupling.
- Component structure must be documented before implementation.

## Decision

- Define separate component-structure documents for CLI and local engine in
  `docs/architecture/`.
- In the local engine, introduce `internal/store` interfaces for names,
  instances, and states, and an `internal/store/sqlite` implementation.
- Persistent data for names/instances/states lives in SQLite under `<StateDir>`.
- Enforce dependency direction: `internal/store/sqlite` depends on
  `internal/store` (interfaces), not the other way around.

## Consequences

- Engine implementation will be refactored into dedicated packages.
- Registry data must be backed by SQLite, not ephemeral in-memory collections.
- The CLI and engine structures are now explicit and reviewed in documentation.
