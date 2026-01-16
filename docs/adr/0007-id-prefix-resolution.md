# ADR 0007: ID prefix resolution and output formatting

Status: Accepted
Date: 2026-01-16

## Decision Record 1

- Timestamp: 2026-01-16T09:18:19+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should sqlrs resolve id prefixes and present ids in output?
- Alternatives:
  - Resolve prefixes in the CLI by listing all objects and filtering locally.
  - Extend the engine API with prefix filters for efficient lookup.
  - Add dedicated prefix lookup endpoints or allow prefix matching in GET-by-id paths.
- Decision: Accept hex id prefixes (8+ chars, case-insensitive) for commands that take ids, and resolve them via engine list endpoints with prefix filters. If a prefix matches multiple ids, treat it as an error. Output ids in lowercase; human output shortens ids to 12 chars by default with `--long` for full ids, while JSON always uses full ids.
- Rationale: CLI-only filtering scales poorly and wastes bandwidth. Prefix filters in list endpoints keep lookup efficient without expanding the path-based GET semantics. Lowercase, short ids improve readability while preserving full ids in machine output.

