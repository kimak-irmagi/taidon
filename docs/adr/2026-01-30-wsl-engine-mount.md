# ADR: WSL btrfs mount is managed by systemd (not the engine)

Status: Accepted (CLI naming superseded by [ADR 2026-02-10: sqlrs init redesign](2026-02-10-sqlrs-init-redesign.md))

- Conversation timestamp: 2026-01-30 08:30 UTC
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the WSL btrfs state store be mounted so Docker and the engine share the same mount namespace?

## Alternatives considered

1. Keep mount logic only in `sqlrs init` and assume the mount namespace is inherited.
2. Run Docker daemon inside WSL and avoid Docker Desktop integration issues.
3. Have the CLI mount the device before each engine start.
4. Have the engine mount the configured device on startup based on workspace config.
5. Install a systemd mount unit in the WSL distro and require systemd to be enabled.

## Decision

Adopt option 5: `sqlrs init --wsl` installs/enables a systemd mount unit and records
its metadata; the engine only validates that the mount is active.

## Rationale

Mount namespaces differ between WSL processes and Docker Desktop. A systemd mount
unit makes the btrfs mount visible to all processes in the distro, including
Docker integration, while keeping the engine thin and avoiding a separate daemon.
