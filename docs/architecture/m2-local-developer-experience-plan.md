# M2 Local Developer Experience Plan

Status: Accepted planning baseline (2026-03-16)

This document defines the implementation plan for the **public/local** part of
roadmap milestone M2. It is intended as the working brief for the engineer who
will implement the feature slices.

The plan deliberately stays on the public/open side of the roadmap. It does not
reintroduce private Team/Shared sequencing or internal backend rollout details.

## 1. Outcome

M2 should reduce local onboarding friction and improve reproducibility tooling
for `sqlrs` users working from a repository on a developer workstation.

The expected public outcome is:

- fewer required flags for common local flows;
- explicit repository-aware workflows driven by user intent;
- reproducible and explainable cache behavior for local runs;
- incremental delivery in slices that remain useful on their own.

## 2. Constraints

- Keep the first M2 slices local-first and CLI-led.
- Prefer CLI-only changes when a slice does not require engine API changes.
- Do not make local workflows depend on hosted/shared infrastructure.
- Reuse the accepted `sqlrs diff` command shape.
- Preserve the current MVP command surface as the stable base.

## 3. Slice Order

The approved implementation order is:

1. repo/workspace conventions
2. shared local input graph primitives
3. `sqlrs diff` first public slice
4. git ref execution baseline
5. provenance and cache explain

This order is chosen to deliver user value early while keeping Git-aware
features bounded and testable.

## 4. Slice Definitions

### 4.1 Slice 1: Repo/Workspace Conventions

**Goal**: reduce configuration friction for common local repository layouts.

**Primary outcome**:

- `sqlrs` can discover conventional prepare inputs from repo/workspace layout;
- local profiles such as `dev` and `test` have a clear documented shape;
- the public docs recommend one or two canonical repository layouts.

**Expected work**:

- define repo layout conventions for `prepare:psql` and `prepare:lb`;
- define config fallback order for discovered prepare inputs;
- document profile conventions and secret-handling boundaries for local use;
- update user guides so the happy path does not start with manual path wiring.

**Suggested CLI surface**:

- no new top-level command is required;
- additions should be expressed as predictable defaults or small flags on
  existing `plan:*` / `prepare:*` commands.

**Tests expected**:

- config/profile resolution tests;
- repo-layout discovery tests;
- negative tests for ambiguous or conflicting layouts;
- user-guide examples aligned with the shipped behavior.

**Out of scope**:

- `sqlrs diff`
- `--ref`
- provenance
- cache explain

### 4.2 Slice 2: Shared Local Input Graph Primitives

**Goal**: establish one deterministic model for revision-sensitive inputs.

**Primary outcome**:

- the CLI can build a deterministic ordered input graph for supported local
  prepare flows;
- the same model can be reused by `diff`, `--ref`, provenance, and cache
  explanation features.

**Expected work**:

- define CLI-side types for context root, input entry, ordered file list, and
  hash material;
- implement a `psql` closure builder for `-f` + include graph;
- implement a Liquibase changelog graph builder for `--changelog-file`;
- define stable hashing and ordering rules;
- keep the model local and file-oriented in the first slice.

**Implementation boundary**:

- CLI-only by default;
- no new engine API unless a clearly justified helper reduces duplication
  without expanding the public protocol.

**Tests expected**:

- deterministic ordering tests;
- include graph closure tests for `psql`;
- changelog graph traversal tests for Liquibase;
- hash stability tests across path normalization cases.

**Out of scope**:

- end-user `diff` command UX
- Git ref access
- provenance output

### 4.3 Slice 3: `sqlrs diff` First Public Slice

**Goal**: ship the first user-visible Git-aware workflow without requiring Git
object access yet.

**Primary outcome**:

- `sqlrs diff` works in `--from-path/--to-path` mode;
- the command wraps exactly one `plan:*` or `prepare:*` invocation;
- users can compare local trees with the same sqlrs command syntax they already
  know.

**Accepted command shape**:

```text
sqlrs diff --from-path <pathA> --to-path <pathB> <wrapped-command> [command-args...]
```

**Expected work**:

- add `diff` command dispatch in the CLI;
- parse diff scope separately from the wrapped command;
- reuse slice-2 input graph builders;
- implement human and JSON diff output;
- document error handling and exit-code behavior.

**Tests expected**:

- argument parsing tests;
- path-mode compare tests for `plan:psql`;
- path-mode compare tests for `prepare:lb`;
- JSON shape tests;
- negative tests for unsupported wrapped commands or mixed scope modes.

**Out of scope**:

- `--from-ref` / `--to-ref`
- wrapped `run:*`
- nested composite `prepare ... run`

### 4.4 Slice 4: Git Ref Execution Baseline

**Goal**: let a user prepare/run from a Git revision without touching the
working tree.

**Primary outcome**:

- `--ref` is usable in the local profile;
- `blob` mode supports zero-copy cache lookup before extraction;
- `worktree` mode exists as the explicit fallback for cases that need it.

**Expected CLI surface**:

```text
sqlrs ... --ref <git-ref> [--ref-mode blob|worktree] [--ref-keep-worktree]
```

**Expected work**:

- resolve repo root and Git refs;
- add blob-mode file access for revision-sensitive inputs;
- add temporary worktree mode;
- integrate cache lookup before file extraction in blob mode;
- define clear user-facing errors for non-Git directories, unresolved refs, and
  missing objects.

**Tests expected**:

- ref parsing and resolution tests;
- blob-mode cache-hit tests;
- worktree lifecycle tests;
- failure tests for missing repo, bad ref, and unsupported inputs.

**Out of scope**:

- hosted/shared repo access
- remote source upload integration
- `sqlrs compare`

### 4.5 Slice 5: Provenance and Cache Explain

**Goal**: make repository-aware local runs reproducible and explainable.

**Primary outcome**:

- a user can persist or print a provenance record for the executed flow;
- a user can ask why a cache lookup was fast, slow, or missed.

**Expected CLI surface**:

```text
sqlrs ... --provenance write|print|both [--provenance-path <path>]
sqlrs cache explain ...
```

**Expected work**:

- define a stable provenance payload for local execution;
- record key run context, input hashes, cache hit/miss decision, and snapshot
  chain identifiers;
- implement a simple `cache explain` report with miss reasons;
- add operator/user guides for interpreting these diagnostics.

**Tests expected**:

- provenance payload tests;
- text/JSON output tests;
- cache-explain tests for hit, partial hit, and miss cases;
- regression tests for missing optional metadata.

**Out of scope**:

- PR automation
- hosted/shared cache introspection
- result comparison between two full execution contexts

## 5. Cross-Slice Rules

- Each slice must produce standalone user value.
- Do not couple the first public slices to new private/shared services.
- Keep command syntax additive and explicit.
- Prefer deterministic local tests over environment-heavy end-to-end coverage
  when the slice is mostly CLI logic.
- When a slice changes command semantics, update both the relevant user guide
  and the architecture/contract docs in the same PR.

## 6. Explicit Non-Goals for This Plan

- Team/Shared backend rollout sequencing
- PR automation or hosted Git integrations
- IDE extension delivery
- `sqlrs compare`
- broad command-surface redesign beyond M2 needs

## 7. Definition of Done for M2

M2 can be considered complete for the public/local roadmap once:

- local repository conventions are documented and implemented;
- `sqlrs diff` exists in its first public path-based slice;
- `--ref` supports a bounded local baseline;
- provenance and cache explanation are available for local repository-aware
  flows;
- the resulting docs describe the shipped public behavior without relying on
  private deployment assumptions.

## 8. References

- [`../roadmap.md`](../roadmap.md)
- [`cli-contract.md`](cli-contract.md)
- [`git-aware-passive.md`](git-aware-passive.md)
- [`diff-component-structure.md`](diff-component-structure.md)
- [`../adr/2026-03-09-git-diff-command-shape.md`](../adr/2026-03-09-git-diff-command-shape.md)
