# ADR: Run recovery for missing instance containers

- Conversation timestamp: 2026-01-25T00:00:00Z
- GitHub user id: @evilguest
- Agent name/version: Codex (GPT-5)
- Question: How should `sqlrs run` behave when the instance container is missing?
- Alternatives considered:
  - Fail the run with an error when the container is missing.
  - Recreate the container from immutable state (`state_id`) and continue.
  - Recreate the container from the instance `runtime_dir` and continue; fail if `runtime_dir` is missing.
- Decision:
  - Recreate the container from `runtime_dir` when the instance container is missing.
  - Keep `runtime_dir` unchanged; update `runtime_id` to the new container id.
  - If `runtime_dir` is missing, fail the run (no fallback to state).
  - Emit run events: `run: container missing - recreating`, `run: restoring runtime`, `run: container started`.
- Rationale:
  - `runtime_dir` contains the most recent mutable instance state, so restoring from it preserves user-visible behavior.
  - Avoids unexpected state rollbacks that could occur when rebuilding from `state_id`.
  - Keeps behavior transparent for CLI users while making missing containers recoverable.
