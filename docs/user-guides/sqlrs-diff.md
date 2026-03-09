# sqlrs diff

## Overview

**Status: design / future.** This command is part of the git-aware passive
feature set described in
[`docs/architecture/git-aware-passive.md`](../architecture/git-aware-passive.md).
It is not implemented in the current MVP CLI.

`sqlrs diff` is a **meta-command**: it wraps one existing content-aware sqlrs
command, evaluates that command in two contexts, and reports the difference.
The goal is to stay syntactically close to the main sqlrs command surface rather
than introducing a separate `diff`-specific input DSL.

In other words, the user writes the command they already know (`plan:*` or
`prepare:*`) and inserts a `diff` block between `sqlrs` and that command.

Use cases:

- Review how a `plan:*` result changes between two branches.
- Review how a `prepare:*` input graph or task body changes between revisions.
- Compare two local trees with the same sqlrs command semantics, without Git.

---

## Command Syntax

```text
sqlrs [global-options] diff (--from-ref <refA> --to-ref <refB> | --from-path <pathA> --to-path <pathB>) <sqlrs-command> [command-args...]
```

Where:

- `diff` defines only the **comparison scope**.
- `<sqlrs-command>` is one wrapped sqlrs command using its **normal syntax**.
- Global flags such as `-v` and `--output` keep their existing meaning.

Examples of wrapped commands:

- `plan:psql`
- `plan:lb`
- `prepare:psql`
- `prepare:lb`

Initial non-goals:

- wrapping a composite `prepare ... run` invocation;
- inventing command-specific output flags such as `--format`;
- changing the syntax of the wrapped command.

---

## Scope Selection

Exactly one scope mode must be used.

### Mode 1: Compare two Git refs

```text
sqlrs diff --from-ref <refA> --to-ref <refB> <sqlrs-command> [command-args...]
```

- `<refA>`, `<refB>` may be `HEAD`, `origin/main`, a commit hash, a tag, or any
  locally resolvable Git ref.
- The wrapped command is evaluated separately at each ref.

### Mode 2: Compare two local paths

```text
sqlrs diff --from-path <pathA> --to-path <pathB> <sqlrs-command> [command-args...]
```

- `<pathA>`, `<pathB>` define the left/right local context.
- The wrapped command is evaluated separately against each local tree.

You must not mix ref-based and path-based scope options.

---

## Wrapped Command Semantics

`sqlrs diff` does not execute the wrapped command in the normal runtime sense.
Instead, it builds the **derived representation** that matters for that command
and compares the two sides.

### `plan:*`

Compare the derived task plan:

- task ordering
- task bodies / hashes
- cacheability-relevant inputs
- any resolved file graph that influences planning

### `prepare:*`

Compare the preparation payload:

- resolved input files
- expanded include graph
- normalized task bodies / content-derived units

### `run:*` (future / limited)

`run:*` is a future extension and is only meaningful for **file-backed inputs**.
Inline-only invocations such as `-c 'select 1'` may be rejected because the Git
revision does not change the payload.

---

## Diff-Specific Options

| Option | Description |
|--------|-------------|
| `--from-ref <ref>` | Left Git revision. |
| `--to-ref <ref>` | Right Git revision. |
| `--from-path <path>` | Left local context. |
| `--to-path <path>` | Right local context. |
| `--ref-mode blob\|worktree` | Ref mode only. How files are loaded from Git. |
| `--include-content` | Include content snippets in human/json output. |
| `--limit <n>` | Truncate listed entries for very large diffs. |

Output selection is **not** a diff-local option:

- `--output human|json` remains the global output selector.
- `-v` / `--verbose` remains the global verbose flag.

---

## Output

### Human output (`--output human`)

- Header: compared scope and wrapped command.
- Added / Modified / Removed sections.
- Optional content snippets when `--include-content` is set.
- Short summary counts.
- Optional semantic hint such as `db_impact=yes|no|unknown` when available.

### JSON output (`--output json`)

A single JSON object with stable top-level fields such as:

- `scope`
- `command`
- `added`
- `modified`
- `removed`
- `summary`
- `db_impact` (optional)

When `--limit` is used, listed entries may be truncated while summary counts
should still represent the full diff when known.

---

## Validation and Errors

- **Missing or conflicting scope** — exactly one of ref mode or path mode must be
  selected.
- **Invalid wrapped command** — `diff` requires one wrapped sqlrs command.
- **Unsupported wrapped form** — nested composite `prepare ... run` is rejected in
  the first slice.
- **Ref resolution failure** — one of the refs does not resolve locally.
- **Not a Git repository** — ref mode requires a Git repository context.
- **No revision-dependent payload** — for future `run:*` support, inline-only
  inputs may be rejected as non-diffable.

---

## Examples

### Compare two refs with `plan:psql`

```bash
sqlrs diff --from-ref origin/main --to-ref HEAD plan:psql -- -f ./prepare.sql
```

### Compare two refs with `prepare:lb`

```bash
sqlrs diff --from-ref origin/main --to-ref HEAD prepare:lb -- \
  update --changelog-file db/changelog.xml
```

### Compare two local trees with `prepare:psql`

```bash
sqlrs --output json diff --from-path ./left --to-path ./right prepare:psql -- -f ./prepare.sql
```

---

## Implementation Notes

1. Parse global flags first, exactly as in a normal sqlrs invocation.
2. Parse the `diff` scope block.
3. Parse the wrapped command using the same grammar and validation rules as the
   main CLI.
4. Evaluate the wrapped command independently on both sides.
5. Perform file discovery and include expansion in each side's own context.
6. Compare the derived representations.
7. Render human or JSON output according to the global `--output` flag.

This command does not start an engine or execute SQL in the first design slice;
it compares the sqlrs-relevant inputs and derived artefacts of the wrapped
command.