# ADR: Podman as an alternative container runtime to Docker

Status: Accepted

- Conversation timestamp: 2026-02-24T17:32:00+07:00
- GitHub user id: @kimak-irmagi
- Agent: Codex (GPT-5)
- Question: Should the local engine support Podman as a first-class container
  runtime alternative to Docker, how should runtime selection be controlled, and
  how should macOS CI prove the Podman execution path?

## Alternatives considered

1. Keep Docker as the only supported runtime.
2. Support Docker/Podman, but control selection only via environment variables.
3. Support Docker/Podman with config-first control and explicit fallback rules;
   keep env vars as an operational override channel.

## Decision

Adopt option 3.

The local engine supports both Docker and Podman CLIs.

Runtime selection follows this model:

1. Engine config (primary user-facing control), with a runtime selector key:
   `container.runtime` where values are `auto|docker|podman`.
2. Environment override for operational/CI scenarios:
   `SQLRS_CONTAINER_RUNTIME` (binary name or path).
3. `auto` fallback order: `docker`, then `podman`.

For Podman, when `CONTAINER_HOST` is not set, the engine may derive it from the
default Podman connection to make runtime commands reachable (notably on macOS).

Status diagnostics must explicitly report runtime probe order and per-runtime
failure causes in verbose mode, so users can see why Podman was attempted.

CI adds a dedicated `macos-latest` Podman e2e probe that runs integration
prepare/run with `container.runtime=podman` set via `sqlrs config`, then
restarts the local engine before execution so runtime selection is applied from
config rather than from env fallback.

## Rationale

This keeps existing Docker-based workflows unchanged while enabling Podman-based
local environments (including macOS setups where Docker may be unavailable or
undesired).  
A config-first contract matches the project's existing control style for
user-facing behavior (schema-backed config with explicit precedence), while env
overrides remain available for ephemeral operational needs.
