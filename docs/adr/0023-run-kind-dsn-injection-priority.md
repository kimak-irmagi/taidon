# ADR 0023: run kind DSN injection priority

- Conversation timestamp: 2026-01-22 22:27:24 +07:00
- GitHub user id: @evilguest
- Agent name/version: Codex (GPT-5)
- Question: What is the preferred DSN injection method for `run:<kind>` commands?
- Alternatives considered:
  - Always inject via environment variables (e.g., `PGHOST`, `PGPORT`, `PGUSER`, `PGDATABASE`).
  - Prefer command-line arguments/positional DSN when supported; use environment variables only when the tool lacks DSN argument support.
  - Per-kind ad hoc rules without a global preference.
- Decision:
  - Prefer command-line arguments or positional DSN injection when the tool supports it.
  - Use environment variables only for run kinds whose tools do not accept DSN via arguments.
- Rationale:
  - Keeps behavior explicit and tool-native, reduces ambiguity, and aligns with `run:psql` (positional DSN) and `run:pgbench` (`-h/-p/-U/-d`).
