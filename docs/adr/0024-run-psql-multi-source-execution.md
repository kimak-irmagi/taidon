# ADR 0024: run:psql multi-source execution strategy

- Conversation timestamp: 2026-01-23 19:20:53 +07:00
- GitHub user id: @evilguest
- Agent name/version: Codex (GPT-5)
- Question: How should `run:psql` handle multiple `-c`, `-f`, or stdin (`-f -`) sources?
- Alternatives considered:
  - Concatenate all sources into a single stdin stream and invoke `psql` once.
  - Execute each `-c`, `-f`, or stdin source as a separate `psql` invocation in order.
  - Reject multiple sources as invalid input.
- Decision:
  - Execute each `-c`, `-f`, or stdin source as a separate `psql` invocation, preserving order.
- Rationale:
  - Aligns with upstream `psql` semantics where each `-c`, `-f`, and stdin source is a separate transaction by default, avoiding unintended transaction merging.
