# ADR 0022: server config management

Status: Accepted
Date: 2026-01-21

## Decision Record 1: store config on the engine

- Timestamp: 2026-01-21T17:40:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Where should server configuration live so retention and future settings cannot be overridden by client configs?
- Alternatives:
  - Keep config on the client and send settings with each request.
  - Store config in the engine, persisting to a JSON file.
  - Store config in SQLite as key/value rows.
- Decision: Keep server config in engine memory and persist it to a JSON file next to the state store.
- Rationale: Prevents client-side misconfiguration from deleting history, keeps local mode simple, and avoids schema churn in SQLite.

## Decision Record 2: general config API and CLI

- Timestamp: 2026-01-21T17:40:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should users read/write server config without adding a new command for every setting?
- Alternatives:
  - Add per-setting CLI flags and endpoints.
  - Introduce a generic config API with JSON paths and a CLI `sqlrs config` command.
  - Keep config file-only and require manual edits.
- Decision: Add a generic config API and CLI commands:
  - `sqlrs config get <path> [--effective]`
  - `sqlrs config set <path> <json_value>`
  - `sqlrs config rm <path>`
  - `sqlrs config schema`
- Rationale: Scales to new settings without redesign and supports both local and remote deployments.

## Decision Record 3: paths, defaults, and validation

- Timestamp: 2026-01-21T17:40:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should config paths, defaults, and validation work?
- Alternatives:
  - XPath-like paths and no defaults.
  - JS-like paths (`a.b[0].c`) with defaults merged at read time.
  - Flat keys only.
- Decision:
  - Use JS-like paths (`a.b.c`, `a.b[42]`).
  - Keep defaults in the engine; `get --effective` returns defaults + overrides.
  - Validate against a built-in JSON Schema; allow extra semantic validation (e.g., non-negative limits).
- Rationale: Familiar path syntax, explicit defaults, and extensible validation.
