# ADR 0017: image digest resolution and resolve_image task

Status: Accepted
Date: 2026-01-20

## Decision Record 1: mandatory digest resolution for base images

- Timestamp: 2026-01-20T20:36:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should base images without a digest be resolved to a digest, and how should this appear in the prepare plan?
- Alternatives:
  - Treat the image id as opaque and use tags as-is.
  - Resolve to digest when available but fall back to tags on failure.
  - Require digest resolution; fail the job if resolution fails; record a `resolve_image` task.
- Decision: Resolve non-digest image references to a canonical digest before planning or execution. If resolution fails, the job fails. Add a `resolve_image` task to the plan when the user did not provide a digest.
- Rationale: Prevents cache collisions across different digests that share a tag, keeps state identification deterministic, and makes long-running resolution visible in task lists.
