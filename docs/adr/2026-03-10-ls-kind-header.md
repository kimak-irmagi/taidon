# ADR: `sqlrs ls` compact `KIND` header in human-readable states table

Status: Accepted
Date: 2026-03-10

## Decision Record 1: use `KIND` as the human-readable states-table header

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should the human-readable `sqlrs ls --states` table keep the long
  `PREPARE_KIND` header, or use a shorter compact header that matches the
  already-short kind aliases shown in the rows?
- Alternatives:
  - Keep `PREPARE_KIND` as the human-readable header.
  - Rename the human-readable header to `KIND` and budget that column for the
    existing short sqlrs kind aliases such as `psql` and `lb`.
- Decision:
  - Use `KIND` as the human-readable states-table column header.
  - Continue rendering the existing short kind aliases (`psql`, `lb`, and future
    aliases of similar shape) in the state rows.
  - Keep JSON and API field names unchanged as `prepare_kind`.
- Rationale: The table is width-sensitive, and the long header wastes horizontal
  space without adding meaning for human readers. `KIND` preserves clarity while
  aligning the header width with the actual compact values shown in the rows.

## Contradiction check

No existing ADR was marked obsolete. This decision refines the compact
human-readable states table and is compatible with the accepted `sqlrs ls`
rendering ADRs from 2026-03-10.
