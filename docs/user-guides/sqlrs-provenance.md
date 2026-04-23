# sqlrs provenance for local prepare flows

## Overview

**Status: proposed next local CLI slice.**

This document proposes the first provenance baseline for repository-aware local
`sqlrs` workflows after the bounded `plan` / `prepare --ref` slice.

The goal is to let a user persist a reproducible execution manifest for the
prepare-oriented command they actually ran, without changing the command's main
stdout/stderr contract.

This first slice stays intentionally narrow:

- supported: single-stage `plan` and `prepare`
- supported: raw and alias-backed prepare flows
- supported: plain local filesystem and bounded local `--ref`
- not supported yet: standalone `run`
- not supported yet: composite `prepare ... run ...`
- not supported yet: remote runner provenance
- not supported yet: provenance print-to-stdout modes

---

## Command Shape

Proposed public syntax:

```text
sqlrs plan [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--provenance-path <path>] <prepare-ref>
sqlrs plan:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--provenance-path <path>] [--image <image-id>] [--] [tool-args...]

sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--provenance-path <path>] <prepare-ref>
sqlrs prepare:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--provenance-path <path>] [--image <image-id>] [--] [tool-args...]
```

Where:

- omitting `--provenance-path` keeps today's behavior unchanged;
- `--provenance-path <path>` requests that sqlrs write one JSON provenance
  artifact for this invocation;
- the path is resolved from the caller's current working directory, not from
  the alias file directory;
- the first provenance slice inherits the same `--ref` rules already accepted
  for bounded local `plan` / `prepare`;
- `prepare --ref --no-watch` stays invalid because that guardrail belongs to the
  bounded local `--ref` slice itself.

---

## Scope of This Slice

### Supported

- `sqlrs plan --provenance-path ./artifacts/chinook-plan.json chinook`
- `sqlrs plan --ref origin/main --provenance-path ./artifacts/chinook-plan.json chinook`
- `sqlrs prepare:psql --provenance-path ./artifacts/prepare.json -- -f ./prepare.sql`
- `sqlrs prepare --ref HEAD~1 --provenance-path ./artifacts/chinook-prepare.json chinook`

### Explicitly out of scope

- `sqlrs run --provenance-path ...`
- `sqlrs prepare ... run ... --provenance-path ...`
- automatic provenance emission without an explicit flag
- human/JSON provenance printing as part of the main command result
- remote/server-side provenance capture

---

## Output Contract

The provenance slice should not change the primary command result shape.

That means:

- `plan` keeps its current human and JSON output;
- `prepare` keeps its current DSN output in watch mode and job-reference output
  in non-ref `--no-watch` mode;
- provenance is written to the requested file as a side artifact, not appended
  to the main stdout payload.

If provenance writing fails after the main command succeeded, the command should
fail and report that write error explicitly instead of silently dropping the
artifact.

---

## Provenance Artifact

The first slice should write one JSON document with enough data to answer:

1. What command shape was executed?
2. Which local input graph and hashes were used?
3. Was a Git ref involved, and if so which resolved revision?
4. Why did sqlrs reuse cache or build new state?
5. What was the terminal outcome?

Minimum fields proposed for the artifact:

- command family and kind (`plan`, `prepare`, `psql`, `lb`, alias vs raw)
- invocation timestamp
- workspace root, caller cwd, and selected alias path when applicable
- selected Git ref metadata when `--ref` is used:
  - requested ref
  - resolved commit
  - ref mode (`worktree` or `blob`)
- normalized prepare input args
- collected input entries with stable content hashes
- cache decision summary:
  - cache key / signature
  - hit vs miss
  - matched state id when present
  - miss reason code when known
- terminal outcome summary:
  - succeeded / failed / canceled
  - plan-only vs prepare execution
  - resulting state id / job id when available

The artifact should avoid ephemeral runtime credentials such as DSNs or auth
tokens. The point is reproducibility and explanation, not secret capture.

---

## Failure Semantics

The baseline should write provenance only after sqlrs has enough bound command
context to describe the intended prepare flow.

That means:

- early usage errors do not emit provenance;
- missing repo / bad ref / missing file errors may emit provenance only if the
  command has already resolved enough context to identify the attempted flow and
  input set;
- execution-time failure after binding should still write provenance with
  `outcome.status = failed`.

This keeps the feature useful for debugging real workflow failures without
forcing a provenance file for trivial parse errors.

---

## Examples

Write provenance for an alias-backed local plan:

```bash
sqlrs plan --provenance-path ./artifacts/chinook-plan.json chinook
```

Write provenance for a ref-backed prepare:

```bash
sqlrs prepare --ref origin/main --provenance-path ./artifacts/chinook-prepare.json chinook
```

Write provenance for a raw SQL prepare:

```bash
sqlrs prepare:psql --provenance-path ./artifacts/prepare.json -- -f ./prepare.sql
```

Not in this slice:

```bash
# not supported in this slice
sqlrs run --provenance-path ./artifacts/run.json smoke --instance dev
```

```bash
# not supported in this slice
sqlrs prepare --provenance-path ./artifacts/composite.json chinook run:psql -- -f ./queries.sql
```

---

## Rationale Summary

This shape keeps the first provenance slice narrow and low-risk:

- one additive file-output flag instead of a new result envelope;
- no change to current stdout JSON/human contracts;
- the same prepare-oriented scope already accepted for bounded local `--ref`;
- enough artifact detail to feed later explanation surfaces such as
  `sqlrs cache explain`.
