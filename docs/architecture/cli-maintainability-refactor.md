# CLI Maintainability Refactor

Status: Accepted (2026-04-16)

## 1. Context

The public/local CLI surface is functionally healthy and well-covered, but the
internal shape has accumulated technical debt from several feature-by-feature
slices:

- `frontend/cli-go/internal/app` now mixes command dispatch, dependency wiring,
  command-context resolution, prepare/run orchestration, cleanup reporting, and
  platform-specific behavior;
- `frontend/cli-go/internal/app/app.go` still depends on package-level mutable
  function variables for testing and dispatch seams;
- alias YAML ownership is split between `internal/app` and `internal/alias`;
- `internal/discover` now has a generic analyzer orchestrator, but its common
  report model still lives inside the aliases analyzer implementation;
- `plan` and `prepare` now share many concepts, but still execute through
  parallel command pipelines with duplicated branching.

This document defines the next refactoring pass for the CLI only. It is
incremental and keeps the public command contract stable.

## 2. Goals

- Preserve current CLI syntax, output shapes, exit-code behavior, and local
  runtime semantics.
- Reduce responsibility concentration in `internal/app`, `internal/alias`, and
  `internal/discover`.
- Create narrower internal seams so later features do not require synchronized
  edits across several oversized files.
- Keep each refactoring slice testable and reviewable as a small PR.

## 3. Non-goals

- No new top-level commands, flags, or output formats.
- No changes to alias-file schema in this refactoring pass.
- No rewrite into a new framework or a large package move in one step.
- No attempt to clean every oversized file immediately.

## 4. Current Debt to Address

### 4.1 `internal/app` owns too many layers

The package currently combines:

- CLI parse + top-level dispatch;
- command-context resolution;
- command-specific option building;
- alias-backed prepare/run orchestration;
- ref-backed prepare binding;
- prepared-instance cleanup reporting;
- WSL and host-command plumbing.

The first visible symptom is the amount of package-level mutable hooks used only
for tests. The deeper issue is that command flow ownership is spread across
several files without one explicit runner boundary.

### 4.2 Alias definition ownership is duplicated

Prepare/run alias YAML loading and validation currently exist in both
`internal/app` and `internal/alias`.

That means schema validation, kind normalization, and file-loading rules are not
owned by one canonical layer, which makes future alias changes riskier than
they need to be.

### 4.3 `discover` generic orchestration still depends on aliases-specific data

`internal/discover/generic.go` already behaves like a shared analyzer
orchestrator, but the common `Report` / `Finding` model still lives in
`internal/discover/aliases.go` and still carries aliases-specific fields.

This blocks clean expansion of generic analyzers and keeps the largest analyzer
file responsible for common discovery contracts.

### 4.4 `plan` and `prepare` still duplicate one stage pipeline

The current `plan` and `prepare` code paths duplicate:

- image resolution;
- prepare-arg parsing and validation;
- input binding;
- ref-backed binding and cleanup;
- kind branching for `psql` and Liquibase.

This makes every cross-cutting change to prepare-oriented flows more expensive
than necessary.

## 5. Refactoring Plan

The work is intentionally staged.

### 5.1 PR1: `internal/app` runner boundary

Introduce an explicit runner/dependency boundary around top-level CLI dispatch.

Status: Implemented in the current branch.

Primary outcome:

- `app.Run(args)` becomes a thin facade;
- package-level mutable hooks used by top-level command dispatch are replaced by
  explicit runner dependencies;
- command dispatch becomes easier to test without package state mutation.

This is the selected first PR.

### 5.2 PR2: canonical alias domain

Move alias definition loading, normalization, and schema validation behind one
canonical owner in `internal/alias`, and make `internal/app` consume that
domain model rather than reading YAML directly.

Status: Implemented in the current branch.

### 5.3 PR3: generic discover model cleanup

Separate generic discovery report types from the aliases analyzer and then split
the aliases analyzer into narrower internal phases such as scan, score,
validate, and suppress.

### 5.4 PR4: shared plan/prepare stage pipeline

Unify the current `plan` and `prepare` execution flow behind one internal stage
pipeline with mode-specific rendering and execution only where they truly
differ.

### 5.5 PR5: optional platform-heavy follow-up

After the previous boundaries are cleaner, split large platform-specific flows
such as `init_wsl.go` into narrower helpers without changing behavior.

## 6. PR1 Design

### 6.1 Scope

PR1 is intentionally narrow.

Included:

- top-level command dispatch in `internal/app`;
- the current `app.Run` facade;
- high-level dependency seams used by dispatch and cleanup reporting;
- command-context creation as a runner collaborator.

Explicitly out of scope for PR1:

- deep prepare-binding hooks such as `bindPreparePsqlInputsFn` and
  `bindPrepareLiquibaseInputsFn`;
- platform-specific shell/host command hooks inside WSL and Btrfs helpers;
- alias schema ownership cleanup;
- discover analyzer data-model cleanup;
- plan/prepare pipeline unification.

The point of PR1 is to establish the top boundary first, not to solve all
package-state usage in one patch.

### 6.2 Target shape

`internal/app` keeps the public entrypoint:

```go
func Run(args []string) error
```

Internally, the package gains an explicit runner object:

- `runnerDeps`
  - owns parser, cwd lookup, command-context resolution, and top-level command
    handlers/collaborators needed by dispatch;
- `runner`
  - owns one invocation and executes the current command sequence using those
    dependencies;
- `newDefaultRunner()`
  - wires the production dependencies once;
- `Run(args)`
  - delegates to the default runner.

Ownership rules:

- `runner` is orchestration-only;
- business logic stays in existing command helpers for now;
- tests construct runners with explicit stub dependencies instead of mutating
  package globals.

### 6.3 Expected file movement

PR1 should mostly touch:

- `frontend/cli-go/internal/app/app.go`
- `frontend/cli-go/internal/app/command_cleanup.go`
- `frontend/cli-go/internal/app/discover.go`
- related `internal/app/*_test.go` files that currently patch top-level hooks

Large behavior moves are not expected in this slice; the main change is who owns
dependency wiring.

### 6.4 Success criteria

PR1 is successful if:

- top-level dispatch no longer depends on mutable package globals in `app.go`;
- tests covering top-level dispatch can stub dependencies through one runner
  boundary;
- command behavior, output, and current public CLI contract remain unchanged.

## 7. PR1 Test Plan

The first implementation slice should add or update tests around the new runner
boundary.

Expected tests:

1. `TestRunnerUsesParserAndReturnsHelpWithoutDispatch`
2. `TestRunnerSkipsCommandContextForInitAndDiff`
3. `TestRunnerBuildsCommandContextOnceForContextualCommands`
4. `TestRunnerRejectsCompositePrepareRefBeforeRunDispatch`
5. `TestRunnerRoutesAliasAndDiscoverThroughInjectedHandlers`
6. `TestRunnerCleansPreparedInstanceThroughInjectedCleanup`
7. `TestRunUsesDefaultRunnerDependencies`

The exact test file split is not important, but the first PR should prove that
top-level dispatch is testable without mutating package-level function state.

## 8. Follow-up Rule

PR1 should not opportunistically absorb PR2-PR4 work. If a change is only
needed to centralize alias ownership, redesign discover payloads, or merge
plan/prepare pipelines, it belongs to a later slice.

## 9. PR2 Design

### 9.1 Scope

PR2 is still intentionally narrow.

Included:

- one canonical execution-facing alias definition model in `internal/alias`;
- shared YAML loading and schema validation for prepare and run aliases;
- filesystem-aware loading so ref-backed prepare alias execution can reuse the
  same loader against a supplied filesystem;
- `internal/app` integration through the alias package instead of local
  duplicate YAML structs and loaders.

Explicitly out of scope for PR2:

- changing alias command syntax or invocation grammar;
- replacing `internal/app` alias argument parsers;
- merging alias path resolution into one shared execution/inspection API when
  that would change current command-specific errors;
- alias create payload redesign;
- discover model cleanup;
- plan/prepare pipeline unification.

The point of PR2 is to establish one canonical alias-definition owner first,
without broadening the patch into every alias-related concern.

### 9.2 Target shape

`internal/alias` becomes the owner of execution-facing alias definitions.

Expected additions:

- `alias.Definition`
  - shared loaded alias metadata:
    - `Class`
    - `Kind`
    - `Image`
    - `Args`
- one shared loader API, exposed from `internal/alias`, for example:
  - `LoadTarget(target Target) (Definition, error)`
  - `LoadTargetWithFS(target Target, fs inputset.FileSystem) (Definition, error)`

Ownership rules:

- `internal/alias` owns YAML loading, kind normalization, and schema checks for
  execution-facing alias files;
- `internal/app` continues to own command-shape parsing such as `prepare`,
  `plan`, and `run` alias invocation flags;
- `internal/app` may keep command-specific path-resolution wrappers in this PR
  if they are still needed to preserve current user-facing errors;
- `CheckTarget` in `internal/alias` must reuse the same shared loader instead of
  maintaining its own duplicate prepare/run definition structs.

After PR2:

- `internal/app` should no longer define duplicate execution-only alias types
  such as separate `prepareAlias` / `runAlias` YAML payload structs;
- `internal/app` should no longer own duplicate `loadPrepareAlias*` /
  `loadRunAlias` functions.

### 9.3 Why path resolution stays split for now

`internal/alias` already owns generic target resolution for inspection and
creation, but `internal/app` still has command-specific wrappers for execution
because current public behavior includes command-specific error wording and a
ref-backed filesystem path for prepare aliases.

Pulling path resolution and domain loading together in one step would enlarge
the refactor and increase the risk of accidental CLI-facing regressions. PR2
therefore centralizes alias definitions first and leaves full execution-path
resolution unification for a later cleanup if it is still needed.

### 9.4 Success criteria

PR2 is successful if:

- one canonical alias-definition loader exists in `internal/alias`;
- alias inspection and alias execution read the same prepare/run schema rules;
- ref-backed prepare alias execution can still load aliases through a supplied
  filesystem;
- `internal/app` no longer duplicates YAML execution models for alias files;
- public CLI syntax, output, and exit-code behavior remain unchanged.

## 10. PR2 Test Plan

The second implementation slice should add or update tests around the shared
alias-definition owner.

Expected tests:

1. `TestLoadTargetPrepareDefinition`
2. `TestLoadTargetRunDefinition`
3. `TestLoadTargetWithFSSupportsPrepareAliasesInRefContexts`
4. `TestLoadTargetRejectsInvalidPrepareSchema`
5. `TestLoadTargetRejectsInvalidRunSchema`
6. `TestCheckTargetReusesSharedAliasDefinitionLoader`
7. `TestResolvePrepareAliasWithOptionalRefLoadsDefinitionsViaAliasPackage`
8. `TestRunAliasExecutionLoadsDefinitionsViaAliasPackage`

The exact test file split is not important, but the PR should prove that
prepare/run execution and alias inspection no longer maintain independent YAML
schema loaders.
