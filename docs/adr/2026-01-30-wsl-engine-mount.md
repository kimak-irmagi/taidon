# ADR: Engine mounts WSL btrfs device on startup

- Conversation timestamp: 2026-01-30 00:00 UTC
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine ensure the WSL btrfs state store is visible to Docker and inherited by child processes?

## Alternatives considered

1. Keep mount logic only in `sqlrs init` and assume the mount namespace is inherited.
2. Run Docker daemon inside WSL and avoid Docker Desktop integration issues.
3. Have the CLI mount the device before each engine start.
4. Have the engine mount the configured device on startup based on workspace config.

## Decision

Adopt option 4: store WSL mount metadata in workspace config and have the engine verify/mount the btrfs device on startup.

## Rationale

Mount namespaces can differ between processes; mounting in the engine process guarantees that all child processes (including Docker) see the same mount. This keeps the CLI thin and avoids requiring a separate Docker daemon inside WSL.
