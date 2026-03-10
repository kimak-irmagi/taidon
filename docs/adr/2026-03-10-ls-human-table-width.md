# ADR: `sqlrs ls` human-readable state table width handling

Status: Accepted
Date: 2026-03-10

## Decision Record 1: keep state rows single-line and truncate `PREPARE_ARGS` in the middle

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs ls --states` render wide `PREPARE_ARGS` values in human-readable output while staying readable in terminals and still supporting redirected output that may be viewed with horizontal scrolling?
- Alternatives:
  - Truncate only the tail of `PREPARE_ARGS` (`prefix ...`).
  - Truncate the middle of `PREPARE_ARGS` (`prefix ... suffix`).
  - Wrap long `PREPARE_ARGS` across multiple physical lines inside the table.
- Decision:
  - Keep human-readable `sqlrs ls --states` output to one physical line per state row.
  - Do not insert hard line wraps inside table cells; emit explicit newlines only between rows.
  - On a TTY, budget `PREPARE_ARGS` against the current terminal width, using the remaining width after the fixed columns with a minimum budget of 16 characters and a maximum budget of 48 characters.
  - If `PREPARE_ARGS` does not fit, truncate it in the middle using the form `prefix ... suffix`.
  - If stdout is not a TTY, do not try to infer the width of the eventual viewer; keep the compact default budget unless the user explicitly requests `--wide`.
  - `--wide` disables `PREPARE_ARGS` truncation in human output, while `--long` continues to control id shortening independently.
- Rationale: Middle truncation preserves both the command prefix and the trailing arguments that often contain the most distinguishing details. Keeping one physical line per state row preserves table scanability, avoids layout breakage when the terminal is resized after the command exits, and makes redirected output predictable for later viewing in editors with horizontal scrolling.

## Contradiction check

No existing ADR was marked obsolete. This decision refines human-readable `sqlrs ls` rendering and is compatible with ADR [2026-03-03-cli-compact-tree-and-image-ids.md](./2026-03-03-cli-compact-tree-and-image-ids.md), which already established compact tree prefixes and shortened `IMAGE_ID` values for readable state tables.
