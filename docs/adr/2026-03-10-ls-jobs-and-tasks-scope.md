# ADR: `sqlrs ls` jobs alignment and deferred task `ARGS`

Status: Accepted
Date: 2026-03-10

## Decision Record 1: align `jobs` with compact state-table rules

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the human-readable `jobs` table relate to the compact formatting rules already adopted for `states`?
- Alternatives:
  - Keep the current `jobs` table mostly independent from `states`.
  - Align `jobs` with the `states` compact policy for `KIND`, `IMAGE_ID`, `PREPARE_ARGS`, and timestamp formatting.
- Decision:
  - Human-readable `jobs` output follows the same compact policy as `states` for `KIND`, `IMAGE_ID`, `PREPARE_ARGS`, and `--long` timestamp behavior.
  - `jobs` also uses a one-character inter-column gap in compact output.
- Rationale: `jobs` describes the same prepare signature surface as `states`, so different formatting rules would create unnecessary inconsistency and wasted width.

## Decision Record 2: improve `tasks` layout now, defer task-specific `ARGS`

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should `tasks` gain a new `ARGS` column in the same slice as the compact table cleanup?
- Alternatives:
  - Add an `ARGS` column immediately using best-effort derivation from existing task fields.
  - Restrict this slice to layout-only improvements (`INPUT` shortening and `OUTPUT_ID`) and defer `ARGS` until a dedicated API field exists.
- Decision:
  - This slice updates only the `tasks` layout: compact `INPUT` rendering and the shorter human-readable header `OUTPUT_ID`.
  - A task-specific `ARGS` column is deferred to a later feature that can introduce an explicit API-level summary field for per-task parameters or subjects.
- Rationale: The current task payload is not sufficient for a stable and general-purpose `ARGS` column across psql and Liquibase tasks. Deferring it avoids encoding fragile heuristics into the CLI surface.

## Contradiction check

No existing ADR was marked obsolete. These decisions refine the `sqlrs ls` human-readable layout while remaining compatible with the accepted compact-table ADRs from 2026-03-10.
