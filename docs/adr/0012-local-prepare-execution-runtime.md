# ADR 0012: local prepare execution runtime

Status: Accepted
Date: 2026-01-19

## Decision Record 1: state store root location

- Timestamp: 2026-01-19T11:27:54.4754632+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Where should the local engine keep runtime state data?
- Alternatives:
  - Use an XDG cache root (for example `~/.cache/sqlrs/state-store`).
  - Place data under the engine state directory.
  - Add a dedicated config flag for the root path.
- Decision: Store runtime state data under `<StateDir>/state-store` for the local engine.
- Rationale: Keeps all local engine state in one place and avoids extra configuration for the MVP.

## Decision Record 2: snapshot backend for the MVP

- Timestamp: 2026-01-19T11:27:54.4754632+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Which snapshot backend should the local engine use first?
- Alternatives:
  - btrfs subvolume snapshots.
  - OverlayFS copy-on-write layers.
  - Full copy snapshots.
- Decision: Start with OverlayFS-based snapshots and fall back to full copy. Add a Windows/WSL backend later.
- Rationale: OverlayFS is widely available on Linux and keeps the MVP simple while preserving CoW benefits.

## Decision Record 3: snapshot consistency and DBMS coordination

- Timestamp: 2026-01-19T11:27:54.4754632+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine coordinate snapshots with Postgres?
- Alternatives:
  - Crash-consistent snapshots without DB coordination.
  - Stop the container completely before every snapshot.
  - Use DBMS-assisted pause and resume inside the running container.
- Decision: Use DBMS-assisted clean snapshots (Postgres `pg_ctl -m fast stop`, snapshot, restart) while the container stays up.
- Rationale: Produces consistent snapshots without full container restarts.

## Decision Record 4: instance lifecycle after prepare

- Timestamp: 2026-01-19T11:35:53.9943137+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What should happen to the container after prepare completes?
- Alternatives:
  - Do not create instances and stop after cache warm-up only.
  - Create the instance but stop the container (cold instance).
  - Create the instance and keep the container running (warm instance).
- Decision: Create the instance and keep the container running after prepare (warm instance).
- Rationale: Keeps the instance immediately available; run orchestration can decide
  when to stop warm instances later.

## Decision Record 5: Postgres runtime assumptions

- Timestamp: 2026-01-19T11:27:54.4754632+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the local engine execute `psql` and configure the Postgres container?
- Alternatives:
  - Execute `psql` inside the container.
  - Execute `psql` on the host.
  - Resolve `PGDATA` dynamically per image.
  - Require explicit credentials.
- Decision: Execute `psql` on the host, assume `PGDATA=/var/lib/postgresql/data`, and set `POSTGRES_HOST_AUTH_METHOD=trust`.
- Superseded: ADR 0016 (execute `psql` inside the container with mounted scripts).
- Rationale: Simplifies file handling and reduces container exec complexity while keeping MVP configuration minimal.
