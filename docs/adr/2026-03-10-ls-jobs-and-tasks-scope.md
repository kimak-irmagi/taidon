# ADR: `sqlrs ls` compact jobs layout and deferred API-driven columns

Status: Accepted
Date: 2026-03-10

## Decision Record 1: align `jobs` with compact state-table rules without extending API

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the human-readable `jobs` table relate to the compact formatting rules already adopted for `states`, given that `PrepareJobEntry` does not currently expose `prepare_args_normalized`?
- Alternatives:
  - Keep the current `jobs` table mostly independent from `states`.
  - Align `jobs` with the `states` compact policy only where the current API already provides the required fields.
  - Extend the API now so `jobs` can also expose `PREPARE_ARGS` in the same slice.
- Decision:
  - Human-readable `jobs` output follows the same compact policy as `states` for `KIND`, `IMAGE_ID`, and `--long` timestamp behavior.
  - `jobs` also uses a one-character inter-column gap in compact output.
- Rationale: `jobs` describes the same prepare signature surface as `states`, so matching headers, id formatting, timestamps, and spacing improves consistency. `PREPARE_ARGS` is deferred because adding it to `jobs` requires an API/client contract extension that should be handled as a separate feature for both `jobs` and `tasks`.

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
