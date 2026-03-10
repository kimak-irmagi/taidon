# ADR: `sqlrs ls` compact timestamps and `--long` time expansion

Status: Accepted
Date: 2026-03-10

## Decision Record 1: use relative `CREATED` in compact output and absolute seconds in `--long`

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs ls --states` render `CREATED` in human-readable output so the compact table remains readable without wasting width on RFC3339 nanoseconds?
- Alternatives:
  - Keep the existing absolute RFC3339Nano timestamp in all human output modes.
  - Keep absolute timestamps, but trim them to second precision in all human output modes.
  - Use an industry-standard relative representation in compact human output and switch to absolute timestamps with second precision in `--long`.
- Decision:
  - Default compact human output renders `CREATED` in a relative form such as `3d ago`, `5h ago`, or `12m ago`.
  - `--long` switches `CREATED` to an absolute UTC timestamp with second precision (RFC3339 without fractional seconds).
  - `--long` therefore expands both ids and timestamps, while `--wide` remains responsible only for disabling `PREPARE_ARGS` truncation.
- Rationale: Relative timestamps are the common operator-facing representation in compact tables and save significant width compared to RFC3339Nano. Keeping absolute timestamps behind `--long` preserves auditability and copy/paste fidelity without forcing that cost on the default table layout.

## Contradiction check

This decision supersedes part of [2026-03-10-ls-human-table-width.md](./2026-03-10-ls-human-table-width.md), which stated that `--long` controlled only id shortening independently of other fields.
