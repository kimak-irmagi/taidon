# sqlrs diff — Component Structure

This document describes the **architectural elaboration** for `sqlrs diff` after
the CLI contract and user guide: which components exist, who calls whom, and
where they live. It is the next step after the design in
[`docs/user-guides/sqlrs-diff.md`](../user-guides/sqlrs-diff.md) and
[`docs/architecture/cli-contract.md`](cli-contract.md).

## 1. Scope and assumptions

- **First slice**: diff runs entirely in the CLI; no new engine API. The CLI
  resolves the two sides (ref or path), builds file lists locally using the
  same closure rules as the main CLI, compares them, and renders output.
- **No engine call** in the first slice: file list construction is implemented
  in the CLI (or by reusing existing engine logic via a future “file list only”
  API if added later). This keeps diff usable without a running engine.
- **Deployment unit**: CLI only (e.g. `frontend/cli-go`). No changes to
  `backend/local-engine-go` required for the first slice.

## 2. Components and responsibilities

| Component | Responsibility | Caller |
|-----------|----------------|--------|
| **Diff command handler** | Parse diff scope and wrapped command; orchestrate scope resolution → file list build (both sides) → compare → render. Map errors to exit codes. | `internal/app` (command dispatch) |
| **Scope resolver** | Given `--from-ref`/`--to-ref` or `--from-path`/`--to-path`, produce two **contexts**. Each context is a root from which to read files: either a Git tree at a ref (blob or temporary worktree) or a local directory. | Diff command handler |
| **File list builder** | For one context and a given **kind** (e.g. psql, lb) plus command args, build the **closure of file inputs**: ordered list of (path, content or hash). Entry point and closure rule are kind-specific (see user guide table). | Diff command handler (called twice: from-context, to-context) |
| **Diff comparator** | Given two file lists (from, to), compute Added / Modified / Removed (by path and optional content hash). Optionally apply `--limit` and `--include-content`. | Diff command handler |
| **Diff renderer** | Turn a diff result into human-readable text or JSON according to global `--output`. | Diff command handler |

## 3. File list builder per kind

The file list builder is the **core abstraction** that differs by kind. Each kind
has a single entry point and a closure rule.

| Kind | Entry point from args | Closure rule | Implementer |
|------|------------------------|--------------|-------------|
| **prepare:psql** / plan:psql | `-f <file>` (and `-f -`) | From each `-f` file, recursively add every file referenced by `\i`, `\ir`, `\include`, `\include_relative`. | `PsqlClosureBuilder` (or shared with engine if reused) |
| **prepare:lb** / plan:lb | `--changelog-file <path>` | From changelog file, add every file referenced by the changelog graph (include, includeAll, etc.). Liquibase defines the graph. | `LbChangelogClosureBuilder` |
| **run:psql** (future) | `-f <file>` (file-backed only) | Same as psql: closure over `\i`/`\include` from `-f`. | Reuse psql closure builder |

The handler selects the builder by the wrapped command’s kind (e.g. `plan:psql` →
psql builder, `prepare:lb` → lb builder). Arguments that are not revision-sensitive
(e.g. `-c`, `--image`) do not affect the file list; the builder only uses the
args that define file-backed inputs.

## 4. Call flow

```text
1. app (command dispatch)
   → detects verb "diff"
   → parses global flags, then diff scope (--from-ref/--to-ref or --from-path/--to-path),
     then wrapped command (e.g. plan:psql -- -f ./x.sql)
   → calls cli.RunDiff(scope, wrappedCommand, globalOptions)

2. RunDiff
   → scopeResolver.Resolve(scope)  →  (fromContext, toContext)
   → fileListBuilder.Build(fromContext, kind, wrappedCommandArgs)  →  fromList
   → fileListBuilder.Build(toContext, kind, wrappedCommandArgs)     →  toList
   → comparator.Compare(fromList, toList, options)                  →  diffResult
   → renderer.Render(diffResult, outputFormat, options)             →  stdout
   → return exit code
```

Scope resolver behaviour:

- **Ref mode**: resolve each ref to a commit/tree; if `--ref-mode blob`, read
  files via Git blobs (e.g. `git show ref:path`); if `worktree`, create a
  temporary worktree and pass its root as the context; optionally `--ref-keep-worktree`
  leaves it for debugging.
- **Path mode**: each path is the context root directly (no Git).

File list builder is chosen by `kind` (psql vs lb). It receives the context root
and the parsed args of the wrapped command, and returns a list of (path, content
or hash) in a deterministic order.

## 5. Suggested package layout (CLI)

All of the following live in the CLI codebase (e.g. `frontend/cli-go`).

| Package | Contents |
|---------|----------|
| `internal/app` | Add diff to command graph; parse diff scope and wrapped command; call `cli.RunDiff`. |
| `internal/cli` | `RunDiff`, diff-specific option types; orchestration of resolver → builder → comparator → renderer. Optionally: human/JSON renderer for diff output. |
| `internal/diff` (new) | `ScopeResolver` (ref + path modes). `FileListBuilder` interface; `PsqlClosureBuilder`, `LbChangelogClosureBuilder`. `Comparator` (Compare). `Renderer` (human + JSON) if not in `internal/cli`. Types: `Context`, `FileList`, `DiffResult`. |

Alternative: keep `internal/diff` minimal (only resolver + comparator + types) and
put closure builders in `internal/cli/diff` or reuse engine-side logic via a small
adapter if the engine exposes a “file list for these args” helper later.

## 6. Data ownership and lifecycle

- **Scope (from/to ref or path)**: parsed once per invocation; not persisted.
- **Contexts**: in-memory representation of the two roots (e.g. worktree path,
  or blob accessor). Temporary worktree, if used, is created before building file
  lists and removed after (unless `--ref-keep-worktree`).
- **File lists**: in-memory for the duration of the command; no cache. Each list
  is an ordered set of (path, content or hash).
- **Diff result**: in-memory; passed to renderer then discarded. No persistent
  state introduced by diff.

## 7. Dependency diagram

```mermaid
flowchart TB
  APP["internal/app <br/> (диспетчер команд)"]
  RUN_DIFF["internal/cli <br/> RunDiff"]
  RESOLVER["internal/diff <br/> ScopeResolver"]
  BUILDER["internal/diff <br/> FileListBuilder<br/>(psql, lb)"]
  COMPARE["internal/diff <br/> Comparator"]
  RENDER["internal/diff <br/> Renderer<br/>(или cli)"]

  APP --> RUN_DIFF
  RUN_DIFF --> RESOLVER
  RUN_DIFF --> BUILDER
  RUN_DIFF --> COMPARE
  RUN_DIFF --> RENDER
  RESOLVER -.->|fromContext, toContext| BUILDER
  BUILDER -.->|fromList, toList| COMPARE
  COMPARE -.->|DiffResult| RENDER
```

## 8. References

- User guide: [`docs/user-guides/sqlrs-diff.md`](../user-guides/sqlrs-diff.md)
- CLI contract: [`docs/architecture/cli-contract.md`](cli-contract.md) (section 3.9)
- Git-aware passive (scenario P3): [`docs/architecture/git-aware-passive.md`](git-aware-passive.md)
- CLI component structure (existing): [`cli-component-structure.md`](cli-component-structure.md)
