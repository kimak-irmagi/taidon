# Job retention for prepare jobs

## Overview

`sqlrs` keeps prepare jobs, tasks, and events so users can inspect recent activity.
To prevent unbounded growth, the engine trims **identical** prepare jobs after
they are queued and planned.

This retention only affects:

- prepare jobs
- prepare tasks
- prepare events

States, instances, and runtime data are not touched.

---

## Job signature (what "identical" means)

Two prepare jobs are considered identical when these values match:

- `prepare_kind`
- `plan_only`
- resolved image id
- normalized prepare args
- input hashes (files/stdin)

In practice, the engine computes a **signature hash** using the same inputs that
build the `state_execute` task hash, plus the resolved image id and `plan_only`.

---

## Retention behavior

When a new prepare job is stored and its tasks are planned:

1. The engine finds completed jobs with the **same signature**.
2. It keeps the newest `N` completed jobs and deletes the rest.
3. Deleting a job removes its tasks and events (cascade).
4. The engine removes the job directory under the state store.

Notes:

- Only **terminal** jobs are trimmed (`succeeded`, `failed`, `cancelled`).
- `running` and `queued` jobs are never deleted.
- Ordering uses `finished_at` (newest first). If `finished_at` is missing,
  `created_at` is used as a fallback.
- Job deletion removes `state-store/jobs/<job_id>` to avoid orphaned folders.

---

## Configuration

Configure the per-signature retention limit in `.sqlrs/config.yaml`.
Values are resolved by the normal config merge rules (global + project).

```yaml
orchestrator:
  jobs:
    maxIdentical: 2
```

Recommended values:

- `2` keeps the latest two completed jobs per signature.
- `0` disables trimming.

---

## Example

If `maxIdentical: 2` and you run the same `prepare:psql` command five times:

- The latest **two completed** jobs remain.
- Older completed jobs are deleted with their tasks and events.
