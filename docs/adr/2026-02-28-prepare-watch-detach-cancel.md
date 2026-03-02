# ADR: prepare watch/detach/cancel interaction model

Status: Accepted
Date: 2026-02-28

## Decision Record 1: CLI interaction model for prepare watch

- Timestamp: 2026-02-28T00:00:00Z
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)
- Question: How should CLI users detach, reconnect, and stop running `prepare`
  jobs with consistent cross-platform behavior?
- Alternatives considered:
  - Keep current behavior (always attached, no detach/reconnect control).
  - Add `watch` and `attach` aliases, plus direct shortcut-driven actions.
  - Add only `watch`, use a unified control prompt opened by `Ctrl+C`, and
    avoid platform-specific keybindings for destructive actions.
- Chosen solution:
  - Introduce `sqlrs watch <job_id>` (no `attach` alias).
  - Support `prepare --watch` (default) and `prepare --no-watch`.
  - In interactive watch mode, `Ctrl+C` opens control prompt:
    `[s] stop  [d] detach  [Esc/Enter] continue`.
  - `stop` requires explicit confirmation (`Cancel job? [y/N]`).
  - Repeated `Ctrl+C` while prompt is open is treated as `continue`.
- Rationale:
  - A single control prompt provides predictable behavior on Linux/macOS/Windows.
  - `s` (`stop`) lowers accidental cancellation risk compared to repeated `c`.
  - Keeping one reconnect command (`watch`) minimizes CLI surface complexity.

## Decision Record 2: detach semantics in composite prepare/run and cancel API shape

- Timestamp: 2026-02-28T00:00:00Z
- GitHub user id: @evilguest
- Agent: Codex (GPT-5)
- Question: What should detach mean for composite `prepare ... run ...`, and
  how should cancellation be represented in API/status models?
- Alternatives considered:
  - Disallow detach for `prepare ... run ...`.
  - Allow detach and continue background run automatically.
  - Allow detach from prepare and explicitly skip the subsequent run phase in
    the current CLI process.
  - Keep `failed` terminal status with `cancelled` error payload only.
  - Add explicit `cancelled` terminal status.
- Chosen solution:
  - For composite `prepare ... run ...`, `detach` means:
    - detach from `prepare` watch,
    - do not start `run` in the same CLI process,
    - exit successfully after printing `job_id` and skip message.
  - Add explicit cancellation endpoint:
    `POST /v1/prepare-jobs/{jobId}/cancel` (idempotent).
  - Keep terminal status model unchanged: cancellation is reported as
    `failed` with `error.code=cancelled` for jobs and tasks.
- Rationale:
  - Composite detach remains explicit and avoids ambiguous background `run`
    behavior.
  - Preserving the existing status enums avoids broad compatibility churn in
    clients and tests.
  - Dedicated cancel endpoint decouples cancellation from delete semantics.
