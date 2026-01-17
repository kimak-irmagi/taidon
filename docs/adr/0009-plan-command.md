# ADR 0009: plan command and plan task output

Status: Accepted
Date: 2026-01-17

## Decision Record 1: plan command vs prepare dry-run

- Timestamp: 2026-01-17T21:48:18+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should planning be a separate `sqlrs plan` command or a `prepare --dry-run` mode?
- Alternatives:
  - Add a dedicated `sqlrs plan:<kind>` command.
  - Add `--dry-run` to `sqlrs prepare:<kind>` and reuse it for planning.
  - Support both and make `prepare --dry-run` an alias for `plan`.
- Decision: Implement a dedicated `sqlrs plan:<kind>` command.
- Rationale: A separate command keeps the semantics explicit (plan vs execute), avoids overloading `prepare` output, and aligns with the CLI contract and future planner expansion.

## Decision Record 2: plan task list, hashes, and instance step

- Timestamp: 2026-01-17T21:48:18+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What should the plan output contain for task structure and caching?
- Alternatives:
  - Return only a summarized plan without per-task entries.
  - Return a task list but omit per-task hashes and cache flags.
  - Return a task list that includes a `plan` task, `state_execute` tasks with hashes/cache, and a final `prepare_instance` task.
- Decision: Return an ordered task list that starts with a `plan` task, includes `state_execute` tasks with `task_hash` and cache indicators, and ends with a `prepare_instance` task. Snapshotting remains part of `state_execute`.
- Rationale: A task list exposes cache behavior and execution boundaries explicitly while keeping future task kinds extensible.

## Decision Record 3: plan API through prepare jobs

- Timestamp: 2026-01-17T22:03:44+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine API expose plan computation?
- Alternatives:
  - Add a separate `GET /v1/prepare-plan` endpoint with request data in query params.
  - Reuse `POST /v1/prepare-jobs` with a `plan_only` flag and return tasks in job status.
- Decision: Reuse `POST /v1/prepare-jobs` with `plan_only: true`, and return plan tasks inside the job status resource.
- Rationale: This keeps the API unified for long-running planning, avoids URL size limits, and allows status tracking if planners become expensive.
