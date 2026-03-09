# ADR: Persist State Sizes For Listings And Eviction

- Conversation timestamp: 2026-03-09
- GitHub user id: @evilguest
- Agent: Codex GPT-5

## Question

How should sqlrs obtain per-state sizes for `sqlrs ls --states` and for bounded-cache eviction ranking?

## Alternatives Considered

1. Measure each state directory on demand in `sqlrs ls` and in eviction.
2. Persist `size_bytes` in state metadata after snapshot creation and require all readers to use only metadata.
3. Persist `size_bytes` after snapshot creation, use metadata by default, and allow eviction to fall back to live measurement when metadata is still missing for legacy states.

## Decision

Use option 3.

The engine will persist `size_bytes` for newly materialized states after snapshot completion. `sqlrs ls --states` and cache diagnostics will read that stored value directly. Eviction will remain metadata-first, but may remeasure a state directory on demand when `size_bytes` is absent or zero for an older cache entry.

## Rationale

This keeps normal listing fast and deterministic, avoids repeated full directory walks in the CLI, and still preserves strict cache enforcement for pre-existing states that do not yet have size metadata.
