# ADR 0003: SQLite schema as source of truth with doc sync

Status: Accepted
Date: 2026-01-14

## Decision Record

- Timestamp: 2026-01-14T17:08:02+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the SQLite schema be managed to avoid duplication between engine code and documentation?
- Alternatives:
  - Keep SQL embedded directly in markdown and update it manually.
  - Embed SQL only in Go code and describe it in prose.
  - Store SQL in a standalone file and sync markdown snippets from it.
- Decision: Use a canonical `schema.sql` file embedded in the engine, with markdown snippets injected by `pnpm run docs:schemas`.
- Rationale: A single source of truth prevents drift and keeps engine initialization aligned with documentation.

## Context

- The local engine needs an SQLite schema for names/instances/states.
- Embedding SQL directly in markdown causes duplication and drift.
- The engine should execute the same schema used in documentation.

## Decision

- Store the canonical schema in
  `backend/local-engine-go/internal/store/sqlite/schema.sql`.
- Embed the schema into the engine binary with `go:embed`.
- Reference schema snippets in docs via ref blocks:
  `<!--ref:sql -->` + markdown link, with the body injected by a script.
- Maintain docs with `pnpm run docs:schemas`, which refreshes the embedded
  snippets between `<!--ref:body-->` and `<!--ref:end-->`.

## Consequences

- Documentation and engine initialization share the same schema source.
- Any schema change requires running `pnpm run docs:schemas`.
