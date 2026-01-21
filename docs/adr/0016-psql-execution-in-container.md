# ADR 0016: psql execution in container

Status: Accepted
Date: 2026-01-20

## Decision Record 1: psql execution and script mounts

- Timestamp: 2026-01-20T15:43:07.6918943+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the local engine execute psql and handle script files?
- Alternatives:
  - Execute `psql` on the host and require a local `psql` binary.
  - Execute `psql` inside the DB container with a bind-mounted script root.
  - Run a separate `psql` runner container and copy files into it.
- Decision: Execute `psql` inside the DB container. The CLI resolves relative `-f/--file`
  paths from its working directory, ensures they stay within the workspace root, and
  sends absolute paths to the engine. The engine mounts the scripts root read-only
  and rewrites file arguments to container paths.
- Rationale: Removes the host dependency, aligns with Windows/WSL behavior, and keeps
  script handling deterministic while still supporting local files.
