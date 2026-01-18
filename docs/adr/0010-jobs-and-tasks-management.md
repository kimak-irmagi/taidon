# ADR 0010: jobs and tasks management

Status: Accepted
Date: 2026-01-18

## Decision Record 1: CLI listing for jobs and tasks

- Timestamp: 2026-01-18T19:58:36+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the CLI list new job and task resources?
- Alternatives:
  - Add separate `sqlrs jobs` and `sqlrs tasks` commands.
  - Add `sqlrs ls --jobs` and `sqlrs ls --tasks` that fetch tasks per job (N+1).
  - Add `sqlrs ls --jobs` and `sqlrs ls --tasks` with a flat task list and `job_id` per task.
- Decision: Extend `sqlrs ls` with `--jobs`/`--tasks` selectors (aliases `-j`/`-t`), include them in `--all`, and show tasks as a flat list with `job_id`.
- Rationale: Keeps listing behavior centralized, avoids per-job API calls, and supports large shared-mode queues.

## Decision Record 2: job deletion via `sqlrs rm`

- Timestamp: 2026-01-18T19:58:36+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should `sqlrs rm` delete jobs and resolve job ids?
- Alternatives:
  - Add a dedicated `sqlrs rm --job <id>` flag.
  - Allow job id prefix matching the same way as state/instance ids.
  - Treat job ids as opaque full ids, resolve alongside state/instance prefixes, and error on ambiguity.
- Decision: `sqlrs rm` accepts full job ids only, resolves them alongside state/instance prefixes, and errors on ambiguity; active jobs (at least one task started) are blocked unless `--force` is set.
- Rationale: Keeps rm a single entry point, avoids accidental deletion via prefix matches, and preserves the safety default while offering an explicit override.

## Decision Record 3: API resources for jobs and tasks

- Timestamp: 2026-01-18T19:58:36+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Which API resources should expose job and task listing and deletion?
- Alternatives:
  - Add per-job task endpoints only and aggregate in the CLI.
  - Introduce a new `/v1/tasks` list endpoint and keep jobs under `/v1/prepare-jobs`.
  - Rename to a generic `/v1/jobs` namespace and migrate prepare jobs there.
- Decision: Add `GET /v1/prepare-jobs` for job listing and `DELETE /v1/prepare-jobs/{jobId}` for job deletion, plus a flat `GET /v1/tasks` endpoint for task listing.
- Rationale: Keeps the existing prepare job resource stable, supports large task queues with a single request, and avoids premature namespace migration.
