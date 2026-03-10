# ADR: bounded cache diagnostics surface

Status: Accepted
Date: 2026-03-09

## Decision Record 1: include compact cache summary in default `sqlrs status`

- Timestamp: 2026-03-09T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should bounded-cache diagnostics be exposed in the CLI so operators get quick cache visibility without paying the full detail cost on every status call?
- Alternatives:
  - Keep cache diagnostics only behind `sqlrs status --cache` and leave default `status` health-only.
  - Include a compact cache summary in default `sqlrs status`, and use `sqlrs status --cache` for full details.
  - Introduce a separate top-level `sqlrs cache ...` command family for MVP hardening.
- Decision: Adopt the two-level `status` model:
  - default `sqlrs status` reports the normal health block plus compact cache indicators (`usage`, effective max, free space, state count, pressure reasons);
  - `sqlrs status --cache` expands that summary into full bounded-cache diagnostics, including thresholds, reclaimability, and last eviction details;
  - `sqlrs ls --states --cache-details` remains the per-state companion view.
- Rationale: This gives operators immediate cache visibility in the command they already use for health checks, while preserving an explicit path for heavier diagnostics and keeping per-state detail separate from the top-level status surface.

## Contradiction check

No existing ADR was marked obsolete. This decision refines the operator-facing diagnostics shape for the accepted bounded-cache policy without changing the underlying capacity or eviction rules.
