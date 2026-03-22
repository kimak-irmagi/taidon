# M2 Local Developer Experience Plan

Status: Accepted planning baseline (2026-03-16)

This document defines the implementation plan for the **public/local** part of
roadmap milestone M2 after the design shifted away from implicit repo-layout
guessing and toward **explicit repo-tracked alias files** plus advisory
workspace discovery.

The plan stays on the public/open side of the roadmap. It does not reintroduce
private Team/Shared rollout sequencing or closed backend details.

## 1. Outcome

M2 should reduce local onboarding friction and improve reproducibility tooling
for `sqlrs` users working from a repository on a developer workstation.

The expected public outcome is:

- repo-tracked workflow recipes for common `plan`, `prepare`, and `run` flows;
- explicit separation between versioned workflow definitions and local-only
  workspace configuration;
- advisory discovery tooling that suggests improvements without becoming part of
  execution semantics;
- deterministic shared CLI-side inputset building blocks that later support
  execution, `diff`, Git-ref mode, provenance, and cache explanation.

## 2. Constraints

- Keep the early M2 slices local-first and CLI-led.
- Do not rely on execution-time guesswork.
- Keep versioned workflow definitions separate from `.sqlrs/config.yaml`.
- Keep aliases and runtime names as separate entities.
- Prefer CLI-only changes until an engine API is clearly justified.
- Keep command syntax additive and explicit.
- Keep one shared CLI-side source of truth for each tool kind's file-bearing
  semantics across execution, `diff`, and `alias check`.

## 3. Approved Slice Order

The approved implementation order is:

1. file-based prepare aliases
2. run aliases, alias inspection, and mixed `prepare ... run` composition
3. `discover --aliases`
4. generic discover analyzers
5. shared local inputset layer
6. `sqlrs diff` path mode
7. git ref execution baseline
8. provenance and cache explain

This order gives immediate public value while keeping future Git-aware work
bounded and testable.

## 4. PR Slices

### 4.1 PR1: File-Based Prepare Alias Baseline

**Goal**: make repo-tracked prepare recipes executable without mixing them into
local workspace config.

**Primary outcome**:

- `sqlrs plan <prepare-ref>` resolves `<prepare-ref>.prep.s9s.yaml` from the
  current working directory
- `sqlrs prepare <prepare-ref>` resolves the same alias class
- exact-file escape via trailing `.` is supported
- runtime names remain separate from alias refs

**Expected work**:

- define prepare alias-file format
- implement current-working-directory-relative alias-ref resolution
- resolve file-bearing alias arguments relative to the alias file directory
- add alias-mode dispatch for `plan` and `prepare`
- document interaction with `--name`

**Tests expected**:

- alias-ref resolution tests
- exact-file escape tests
- prepare alias validation tests
- negative tests for missing files and kind/schema errors

**Out of scope**:

- run aliases
- discover
- diff
- Git refs

### 4.2 PR2: Run Aliases, Alias Inspection, and Mixed Composition

**Goal**: complete the explicit alias execution surface and make normal
`prepare ... run` pipelines work across raw and alias modes.

**Primary outcome**:

- standalone `sqlrs run <run-ref> --instance <id|name>` resolves
  `<run-ref>.run.s9s.yaml` from the current working directory
- `prepare ... run ...` accepts mixed raw/alias combinations
- `sqlrs alias ls [--prepare] [--run] [--from <workspace|cwd|path>] [--depth <self|children|recursive>]`
- `sqlrs alias check [--prepare] [--run] [--from <workspace|cwd|path>]`
  `[--depth <self|children|recursive>] [<ref>]`

**Expected work**:

- define run alias-file format
- add run alias-mode dispatch
- allow raw/alias stage mixing in composite `prepare ... run`
- forbid `--instance` when the target instance is already selected by a
  preceding `prepare`
- implement alias listing/check commands with bounded scan-scope controls
- keep raw `run:<kind>` mode intact alongside alias mode

**Tests expected**:

- run alias resolution tests
- composite grammar tests for mixed raw/alias stages
- alias inspection command tests
- scan-root and scan-depth tests for `alias ls` / `alias check`
- single-alias `check <ref>` tests, including ambiguity and exact-file escape
- validation tests for kind-specific constraints
- negative tests for missing `--instance`, conflicting composite `--instance`,
  and wrong alias type

**Out of scope**:

- discover analyzers
- diff
- Git refs

### 4.3 PR3: `discover --aliases`

**Goal**: help repository authors bootstrap explicit alias files without making
execution depend on heuristics.

**Primary outcome**:

- `sqlrs discover --aliases` analyzes the workspace and reports candidate alias
  files
- the command is advisory and read-only by default

**Expected work**:

- add the `discover` command family
- implement the `--aliases` analyzer
- define stable human and JSON output for findings
- keep discovery separate from execution semantics

**Tests expected**:

- analyzer selection tests
- candidate detection tests
- JSON finding-shape tests
- regression tests proving `plan/prepare/run` do not fall back to discovery

**Out of scope**:

- generic discover analyzers
- write mode for unrelated workspace files
- diff

### 4.4 PR4: Generic Discover Analyzers

**Goal**: turn `discover` into a general advisory workflow for local repository
hygiene and cache-friendly shaping.

**Primary outcome**:

- baseline analyzer framework for multiple selectors
- initial non-alias analyzers such as:
  - `--gitignore`
  - `--vscode`
  - `--prepare-shaping`

**Expected work**:

- add analyzer registration and selection rules
- define shared finding structure where practical
- keep analyzer-specific semantics explicit

**Tests expected**:

- multi-analyzer selection tests
- per-analyzer finding tests
- negative tests for incompatible write/update modes if introduced

**Out of scope**:

- Git-ref workflows
- provenance

### 4.5 PR5: Shared Local InputSet Layer

**Goal**: establish one shared CLI-side source of truth for revision-sensitive
local inputs.

**Primary outcome**:

- the CLI has one shared `inputset` layer for supported file-bearing tool kinds
- the same layer is reused by `prepare`, `plan`, `run`, `alias check`, `diff`,
  discover analyzers, Git-ref mode, provenance, and cache explanation

**Expected work**:

- define CLI-side parse/bind/collect/project abstractions plus shared resolver
  and filesystem types
- implement the shared `psql` inputset component
- implement the shared Liquibase inputset component
- implement the shared `pgbench` inputset component for file-bearing run inputs
- wire execution and alias-check flows to the shared layer
- define stable hashing and ordering rules

**Tests expected**:

- parser/binding parity tests for `psql`, Liquibase, and `pgbench`
- deterministic ordering tests
- closure traversal tests for `psql`
- changelog traversal tests for Liquibase
- hash stability tests across normalization cases
- projection parity tests across execution and inspection consumers

### 4.6 PR6: `sqlrs diff` Path Mode

**Goal**: ship the first user-visible Git-aware workflow without requiring Git
object access yet.

**Status (partially delivered in `frontend/cli-go`)**: path mode and ref mode
(worktree) work for a **single** wrapped `plan:psql` / `plan:lb` / `prepare:psql`
/ `prepare:lb`; comparison is **file-list closures + hashes**, no engine.
**Still open** vs this PR’s original wording: two-stage `prepare ... run`
composite parsing, alias `prepare <ref>`, JSON/human **per-phase** layout, and
migration from diff-owned builders to the shared PR5 `inputset` layer.

**Primary outcome** (target):

- `sqlrs diff --from-path/--to-path ...` works for one wrapped `plan:*` or
  `prepare:*` command, and for the normal two-stage `prepare ... run`
  composite grammar

**Expected work**:

- add `diff` command dispatch
- parse diff scope separately from the wrapped command
- reuse PR5 shared inputset components instead of diff-owned parsers/builders
- evaluate wrapped `prepare` and `run` phases separately when a composite is
  used
- implement human and JSON rendering

**Tests expected**:

- argument parsing tests
- path-mode compare tests for `plan:psql`
- path-mode compare tests for `prepare:lb`
- parity tests showing `diff` uses the same per-kind input semantics as the
  shared execution layer
- path-mode compare tests for mixed raw/alias `prepare ... run`
- JSON shape tests

### 4.7 PR7: Git Ref Execution Baseline

**Goal**: let a user execute repository-aware workflows from a Git revision
without touching the working tree.

**Primary outcome**:

- bounded local `--ref` support
- `blob` mode with zero-copy cache lookup before extraction
- explicit `worktree` fallback mode

**Expected work**:

- Git ref resolution
- blob-mode input access
- worktree lifecycle handling
- clear user-facing errors for non-Git and missing-object cases

**Tests expected**:

- ref parsing and resolution tests
- blob-mode cache-hit tests
- worktree lifecycle tests
- failure tests for bad refs and missing objects

### 4.8 PR8: Provenance and Cache Explain

**Goal**: make repository-aware local workflows reproducible and explainable.

**Primary outcome**:

- provenance output for local alias/Git-ref workflows
- `sqlrs cache explain ...` for user-facing hit/miss diagnostics

**Expected work**:

- define provenance payload
- record input hashes and cache decisions
- implement human and JSON cache-explain output

**Tests expected**:

- provenance payload tests
- cache-explain hit/miss tests
- text/JSON rendering tests

## 5. Cross-Slice Rules

- Each slice must deliver standalone user value.
- Discovery remains advisory unless a specific write mode is explicitly selected.
- Execution commands must never depend on prior `discover` output.
- Alias refs stay deterministic and current-working-directory relative.
- File-bearing paths inside alias files stay relative to the alias file
  directory.
- Names remain runtime handles and do not replace aliases.
- When a slice changes command semantics, update the relevant user guide and
  architecture/contract docs in the same PR.

## 6. Explicit Non-Goals for This Plan

- Team/Shared backend rollout sequencing
- PR automation or hosted Git integrations
- IDE extension delivery itself
- `sqlrs compare`
- large command-surface redesign beyond the alias/discover/Git-aware roadmap

## 7. Definition of Done for M2

M2 can be considered complete for the public/local roadmap once:

- repo-tracked prepare/run aliases exist and are executable;
- `discover` provides useful advisory analysis for local repositories;
- `sqlrs diff` exists in path mode;
- Git-ref execution is supported in a bounded local baseline;
- provenance and cache explanation are available for repository-aware flows;
- the resulting docs describe the shipped public behavior without relying on
  private deployment assumptions.

## 8. References

- [`../roadmap.md`](../roadmap.md)
- [`../user-guides/sqlrs-aliases.md`](../user-guides/sqlrs-aliases.md)
- [`cli-contract.md`](cli-contract.md)
- [`git-aware-passive.md`](git-aware-passive.md)
- [`diff-component-structure.md`](diff-component-structure.md)
