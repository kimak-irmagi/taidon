# ADR 0019: persist runtime dir for instance cleanup

Status: Accepted
Date: 2026-01-21

## Decision Record 1: store runtime_dir on instances

- Timestamp: 2026-01-21T10:20:00+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine avoid leaking per-job runtime directories after prepare/instance deletion?
- Alternatives:
  - Store `job_id` and re-derive the path under `<StateDir>/jobs/<job_id>/runtime`.
  - Store the absolute `runtime_dir` path with the instance and delete it on instance removal.
- Decision: Persist `runtime_dir` on instances and delete it when the instance is removed.
- Rationale: Keeps cleanup robust to future path layout changes and avoids leaking runtime data between prepare/delete cycles.
