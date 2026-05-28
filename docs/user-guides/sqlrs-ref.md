# sqlrs `--ref` for plan/prepare

## Overview

**Status: approved bounded local CLI design.**

This document defines the next Git-aware local slice after `sqlrs diff`: allow
`plan` and `prepare` to read their repository-backed inputs from a selected Git
revision without changing the caller's working tree.

This slice is intentionally narrow:

- supported: single-stage `plan` and `prepare`
- supported: raw and alias-backed prepare flows
- standalone `run --ref` is designed separately in
  [`sqlrs-run-ref.md`](sqlrs-run-ref.md)
- not supported yet: composite `prepare ... run ...` with `--ref`
- not supported yet: provenance and `cache explain`

The goal is to ship a bounded local `--ref` baseline before widening into
explanation features or multi-stage revision semantics.

---

## Command Shape

Proposed public syntax:

```text
sqlrs plan [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] <prepare-ref>
sqlrs plan:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--image <image-id>] [--] [tool-args...]

sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] <prepare-ref>
sqlrs prepare:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--image <image-id>] [--] [tool-args...]
```

Selection rules:

- omitting `--ref` keeps today's filesystem behavior unchanged;
- `--ref` is local-only and requires a Git repository context;
- `--ref-mode` and `--ref-keep-worktree` are valid only when `--ref` is set;
- `--ref-mode` defaults to `worktree`;
- `--ref-keep-worktree` is valid only with `--ref-mode worktree`.
- `prepare --ref --no-watch` is rejected in this first slice; ref-backed
  prepare remains watch-only.

The flag belongs to the `plan` / `prepare` stage itself, not to global CLI
options.

---

## Scope of This Slice

### Supported in this slice

- `sqlrs plan --ref <ref> <prepare-alias>`
- `sqlrs plan:psql --ref <ref> -- -f ...`
- `sqlrs plan:lb --ref <ref> -- update --changelog-file ...`
- `sqlrs prepare --ref <ref> <prepare-alias>`
- `sqlrs prepare:psql --ref <ref> -- -f ...`
- `sqlrs prepare:lb --ref <ref> -- update --changelog-file ...`

For `prepare`, this support is intentionally limited to watch mode.

### Explicitly out of scope

- standalone `run --ref ...` in this document (see
  [`sqlrs-run-ref.md`](sqlrs-run-ref.md))
- `sqlrs prepare ... run ...` when the prepare stage carries `--ref`
- `sqlrs prepare --ref --no-watch ...`
- `sqlrs diff` syntax changes
- remote runner semantics
- automatic provenance emission
- `sqlrs cache explain`

The main reason to defer composite `prepare ... run ...` here is to avoid
mixing one revision-sensitive prepare stage with a second run stage whose alias
file or file-backed inputs would otherwise need separate per-stage revision
rules in the same PR.

---

## Revision Context Rules

When `--ref` is present, sqlrs evaluates the `plan` or `prepare` stage inside a
projected repository context:

1. Find the repository root from the caller's current working directory.
2. Resolve `--ref <git-ref>` locally.
3. Project the caller's current working directory into that revision, exactly as
   `sqlrs diff` already does for ref mode.
4. Resolve the command's file-bearing inputs inside that projected revision
   context.

This means the current working directory still matters.

Example:

- repo root: `/repo`
- caller cwd: `/repo/examples/chinook`
- command: `sqlrs prepare --ref origin/main app`

Then alias resolution starts from the projected directory
`<origin/main>:/examples/chinook`, not from the repository root.

If that projected cwd does not exist at the selected ref, the command fails.

---

## Alias Mode Under `--ref`

Alias mode keeps the same logical rules as today; only the filesystem backing
changes from the live working tree to the selected revision.

For:

```text
sqlrs plan --ref <git-ref> <prepare-ref>
sqlrs prepare --ref <git-ref> <prepare-ref>
```

rules are:

- `<prepare-ref>` remains a cwd-relative logical stem;
- exact-file escape via trailing `.` still works;
- the alias file is resolved inside the projected ref context;
- file-bearing paths read from that alias file remain relative to the alias file
  directory inside the same ref context.

Example:

- caller cwd: `<repo>/examples`
- command: `sqlrs plan --ref HEAD~1 chinook`
- resolved alias file: `<HEAD~1>:/examples/chinook.prep.s9s.yaml`

If the alias file exists in the current working tree but not at the selected
ref, the command fails explicitly.

---

## Raw Mode Under `--ref`

For raw `plan:<kind>` and `prepare:<kind>` invocations:

- file-bearing arguments are still interpreted using the same per-kind rules as
  today;
- relative paths are resolved from the projected cwd at the selected ref;
- the shared `internal/inputset` semantics remain the source of truth for file
  discovery and closure collection.

Examples:

```bash
sqlrs plan:psql --ref origin/main -- -f ./prepare.sql
sqlrs prepare:lb --ref HEAD~1 -- update --changelog-file db/changelog.xml
```

Both commands use the selected revision as the filesystem backing for those
paths and any included/dependent files they reference.

---

## Ref Modes

The mode flags match the existing `sqlrs diff` contract.

### `--ref-mode worktree` (default)

- materialize the selected revision as a detached temporary worktree;
- project the caller's cwd into that worktree;
- evaluate plan/prepare inputs against normal filesystem semantics;
- remove the temporary worktree after the command unless
  `--ref-keep-worktree` is set.

This is the default because it preserves the closest behavior to today's local
filesystem execution, including symlink-sensitive cases.

### `--ref-mode blob`

- read files directly from Git objects without creating a detached worktree;
- preserve the same projected-cwd model logically;
- rely on the shared Git-backed filesystem layer for file reads and directory
  traversal.

This mode is lighter but remains opt-in because `worktree` is the safer default
when full filesystem behavior matters.

---

## Output and UX

This slice does not introduce a new top-level output mode.

Successful `plan` and `prepare` output should remain the same shape they have
today:

- `plan` keeps its existing human/JSON structure;
- `prepare --ref` stays in watch mode and keeps DSN output;
- plain `prepare` without `--ref` still supports `--no-watch` and returns job
  references.

Ref context is surfaced through:

- normal validation errors when ref resolution or projected-path lookup fails;
- verbose logs (`-v`) that show the selected ref and ref mode;
- future provenance work, not this slice.

No separate `source_context` payload is proposed in this slice.

---

## Validation and Errors

New validation rules:

- `--ref` requires a Git repository context;
- `--ref-mode` without `--ref` is a usage error;
- `--ref-keep-worktree` without `--ref` is a usage error;
- `--ref-keep-worktree` with `--ref-mode blob` is a usage error;
- `prepare --ref --no-watch` is a usage error in this slice;
- the selected ref must resolve locally;
- the caller's projected cwd must exist at that ref;
- the selected alias file or raw file-bearing entrypoint must exist at that ref.

Examples of user-facing failures:

- not a Git repository
- unknown ref
- projected cwd missing at ref
- alias file missing at ref
- `-f` / `--changelog-file` target missing at ref
- missing included file inside the ref-backed file graph

These should remain regular command errors, not discover findings.

---

## Examples

Plan an alias-backed workflow from another revision:

```bash
sqlrs plan --ref origin/main chinook
```

Prepare from a raw SQL file at another revision:

```bash
sqlrs prepare:psql --ref HEAD~1 -- -f ./prepare.sql
```

Prepare from a Liquibase changelog without touching the current working tree:

```bash
sqlrs prepare:lb --ref feature/new-schema -- update --changelog-file db/changelog.xml
```

Keep the temporary worktree for debugging:

```bash
sqlrs plan --ref origin/main --ref-keep-worktree chinook
```

Use direct Git-object reads:

```bash
sqlrs plan:psql --ref origin/main --ref-mode blob -- -f ./prepare.sql
```

Not in this slice:

```bash
# rejected in this slice
sqlrs prepare --ref origin/main chinook run:psql -- -f ./queries.sql
```

```bash
# rejected in this slice
sqlrs prepare --ref origin/main --no-watch chinook
```

---

## Rationale Summary

This CLI shape keeps the next Git-aware slice bounded:

- one explicit `--ref` flag reused by `plan` and `prepare`;
- the same `worktree` vs `blob` vocabulary already established by `diff`;
- no changes to successful `plan`/`prepare` output shape yet;
- `prepare --ref` remains watch-only so async ref-backed prepare semantics stay
  out of this first PR;
- no mixed-stage revision semantics in the same PR;
- alias mode and raw mode keep the same path-base rules they already have today.

If approved, the next design step should move from this CLI shape into
interaction flow and internal component structure for local ref-backed
plan/prepare execution.
