# sqlrs watch

## Overview

`sqlrs watch` attaches to an existing **prepare job** and streams its progress.

It is used when a job was started earlier (for example with `sqlrs prepare --no-watch`)
or when the user detached from watch mode and wants to reconnect.

---

## Command Syntax

```text
sqlrs watch <job_id>
```

Where:

- `<job_id>` is a full prepare job id (no prefix matching).

---

## Behavior

- The command opens the job events stream and renders progress in the same format
  as `sqlrs prepare --watch`.
- On status events, the CLI re-fetches `GET /v1/prepare-jobs/{jobId}`.
- The command exits on terminal status: `succeeded` or `failed`.
- Cancellation is reported as `failed` with `error.code=cancelled`.

---

## Interactive Controls

In interactive terminals, `Ctrl+C` opens the control prompt:

```text
[s] stop  [d] detach  [Esc/Enter] continue
```

Rules:

- `s` asks confirmation (`Cancel job? [y/N]`) and then sends a cancel request.
- `d` detaches from watch and leaves the job running.
- `Esc` or `Enter` resumes watch.
- Repeated `Ctrl+C` while prompt is open is treated as `continue`.

---

## Output

- Human mode prints progress events to stderr and exits with terminal status.
- JSON mode is reserved for a future extension.

---

## Errors

`sqlrs watch` may fail with:

- job not found
- unauthorized
- events stream errors
- stream completion without definitive terminal status
