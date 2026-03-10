# ADR: `sqlrs ls` API surface for jobs and tasks

Status: Accepted
Date: 2026-03-10

## Decision Record 1: extend `jobs` list payload with resolved image and normalized args

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs ls --jobs` become semantically consistent with `states` once the compact table starts showing image ids and prepare signatures in the same style?
- Alternatives:
  - Keep `PrepareJobEntry` limited to the original requested `image_id` and omit `PREPARE_ARGS`.
  - Extend `PrepareJobEntry` with `prepare_args_normalized` only.
  - Extend `PrepareJobEntry` with both `prepare_args_normalized` and `resolved_image_id`, while preserving the original `image_id`.
- Decision:
  - `PrepareJobEntry` exposes the original requested `image_id`, optional `resolved_image_id`, and optional `prepare_args_normalized`.
  - Human-readable `ls --jobs` prefers `resolved_image_id` for `IMAGE_ID` when it is present, and falls back to `image_id` otherwise.
  - Human-readable `ls --jobs` renders `PREPARE_ARGS` as a width-budgeted wide column in compact output and fully expands it under `--wide`.
- Rationale: `states` and `jobs` describe the same preparation signature from different lifecycle stages. Showing only the requested image ref in `jobs` makes the table look consistent while actually representing a different semantic value than `states`.

## Decision Record 2: add an explicit task summary field instead of CLI heuristics

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs ls --tasks` expose per-task argument or subject information without baking unstable parsing rules into the CLI?
- Alternatives:
  - Derive `ARGS` in the CLI from existing task fields using best-effort heuristics.
  - Expose only Liquibase-specific fields and keep psql tasks without summaries.
  - Add an explicit API field such as `args_summary` and populate it in the engine from task planning metadata.
- Decision:
  - `TaskEntry` gains optional `args_summary`.
  - Human-readable `ls --tasks` renders `ARGS` from that field and treats it as a width-budgeted wide column controlled by `--wide`.
  - The engine remains free to compute that summary from task-kind-specific metadata, but the CLI consumes one explicit stable field.
- Rationale: Liquibase tasks already have structured changeset metadata, while psql step identity is currently planner-internal. A dedicated summary field avoids fragile CLI-side reverse engineering and gives one stable contract to human output and JSON tooling.

## Decision Record 3: render image task inputs with the image formatter

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should compact `ls --tasks` render `INPUT` for image-based tasks when the input id is already a resolved image reference?
- Alternatives:
  - Shorten all task input ids with the generic opaque-id formatter.
  - Use kind-aware formatting: generic id shortening for states and digest-aware image formatting for image inputs.
- Decision:
  - Compact task `INPUT` rendering is kind-aware.
  - `state` inputs use the regular id shortener.
  - `image` inputs use the same digest-aware formatter as `IMAGE_ID` columns.
- Rationale: Image task inputs can already contain resolved digest references. Treating them like opaque ids collapses distinct digests from the same repository into nearly identical compact values and weakens debugging output.

## Contradiction check

This ADR supersedes the "defer" direction from [2026-03-10-ls-jobs-and-tasks-scope.md](./2026-03-10-ls-jobs-and-tasks-scope.md) for `jobs PREPARE_ARGS` and task `ARGS`. The earlier ADR remains historically accurate for the pre-API-extension slice.
