# ADR 2026-02-02: Liquibase per-changeset execution uses update-count

Status: Accepted
Date: 2026-02-02

## Decision Record

- Timestamp: 2026-02-02T20:45:00+00:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Which Liquibase command should `prepare:lb` use to apply one changeset at a time?
- Alternatives:
  - Use `updat-one-changeset` with `--changeset-id/author/path`.
  - Use `update-count --count=1` to apply the next pending changeset.
- Decision:
  - Use `update-count --count=1` for per-changeset execution.
- Rationale:
  - `update-one-changeset` requires Liquibase Secure license.
  - `update-count --count=1` is documented and applies exactly one pending changeset, matching the stepwise snapshot model.
