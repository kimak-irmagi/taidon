# Git-aware semantics: active features (GitHub/Git webhooks and bot)

Status: **proposed future design**. Git-aware flags and "prepare-only run"
behaviors referenced here are not available in the current MVP CLI. Today, the
MVP uses a composite invocation such as `sqlrs prepare:psql ... run:psql ...`.

Goal: deliver a "wow effect" via automation around PRs/branches **without forcing** a new workflow. This document covers features triggered **by GitHub/Git events** or by PR commands (slash commands).

## Design principles

- **Opt-in by default.** Automation is enabled by repo settings or by a PR label/command.
- **No secrets in PR.** DSN/passwords are never posted to PRs; DSN is obtained locally via `sqlrs`.
- **Remote mode requires repo access.** Ref-based actions require a server-side mirror or VCS secrets.
- **Warmup first, service second.** The primary value is having environments ready when needed (review/QA).
- **Result is an artifact.** Every automation run leaves links to logs and provenance.

---

## Scenario A1. Auto-warm cache on PR (opened/synchronize)

### Motivation

The most painful UX is "the first run after a PR push takes seconds/minutes". Pre-warming makes review and QA instant.

### Behavior (bot/webhook)

Triggers:

- PR opened
- PR synchronize (new commits)
- (optional) PR reopened

Opt-in conditions (variants):

1. PR has label `taidon:warmup`
2. Repository rule "warmup always" is enabled
3. PR comment command `/taidon warmup`

Actions:

1. Determine PR `head SHA` (and optional `base SHA`).
2. Build a list of "warmup contexts":
   - from repo config (if present)
   - from explicit patterns (e.g., `migrations/**`)
   - from PR command (passes `--prepare <path>`)
3. For each context, compute state signature and warm the snapshot chain via a normal `sqlrs run` without a user command (prepare-only).
4. Write provenance + logs.
5. Publish result:
   - GitHub Check Run: "DB warmup: success/fail"
   - (optional) PR comment

### PR result format

- Status: ?/?
- Warmup time
- Warmed contexts (prepare paths)
- How to reproduce locally (CLI example)

### Implementation sketch

- GitHub App / webhook receiver
- Job queue (to avoid "storms")
- For each job:
  1. fetch git objects (no full checkout)
  2. compute hashes for prepare context
  3. call `sqlrs run` in prepare-only mode
  4. store artifacts

---

## Scenario A2. PR commands (slash commands)

### Motivation

Not everyone needs warmup always. PR commands are the least intrusive: the user asks for action when appropriate.

### UX (PR commands)

- `/taidon warmup --prepare <path>`
- `/taidon diff --from-ref base --to-ref head --prepare <path>`
- `/taidon compare --from-ref base --from-prepare <path> --to-ref head --to-prepare <path> --run "psql -c ':'"` (limited)

Security rules:

- only for maintainers or allowlisted users
- secret redaction in output
- rate limiting

### Algorithm

- parse command -> job
- PR context (base/head SHA)
- run the matching passive CLI in an isolated runner
- publish result as PR comment or Check Run

---

## Scenario A3. Taidon-aware diff as GitHub Check

### Motivation

Reviewers want a quick signal: "does this PR change the DB?" and "which migrations are touched?".

### Behavior

Trigger:

- PR opened/synchronize

Action:

- `sqlrs diff --from-ref <base> --to-ref <head> --prepare <path>` for each context

Result:

- check summary:
  - DB impact: yes/no/unknown
  - Added/Modified/Removed changesets
  - link to full report (artifact)

### Algorithm

- same pipeline as A1, but a light task without running DB

---

## Scenario A4. Eviction/retention hints from Git events

### Motivation

Snapshot storage is limited. Git events provide semantic milestones that are worth keeping longer.

### Policy (proposed)

- Keep longer:
  - `main@HEAD` (last successful)
  - release tags `v*`
  - merge-base of popular PRs (if tested often)
- Keep less:
  - temporary for closed PRs
  - old head commits without activity

### Behavior

Triggers:

- push to main
- tag created
- PR closed/merged

Actions:

- set priority/TTL metadata for matching snapshots/state signatures
- (optional) pin "golden snapshots" after release

### Algorithm

- evictor receives events and updates metadata:
  - `priority`, `pin`, `ttl`
- periodic cleanup respects these markers

---

## Scenario A5. PR auto-comments (careful)

### Motivation

A comment is useful only if it is:

- short
- updated (not spammy)
- provides a concrete action ("how to run")

### Publication policy

- default: Check Run only
- comment: only on failure or by user command
- edit a single "pinned" bot comment

### Comment template (example)

- ? Warmup OK (N contexts)
- How to reproduce:
  - `sqlrs run --ref <head> --prepare <path> -- <cmd>`
- Links: logs, provenance

---

## Minimal MVP for active features

1. Slash command `/taidon warmup --prepare <path>`
2. Check Run "Taidon-aware diff" for PR (opt-in label)
3. Auto-warm on PR synchronize when label is present
4. TTL/priority hints for eviction on events: merge, tag, PR closed
