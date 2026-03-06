# sqlrs diff

Russian: [sqlrs-diff.RU.md](sqlrs-diff.RU.md)

## Overview

**Status: design / future.** This command is part of the git-aware passive feature set
described in [`docs/architecture/git-aware-passive.md`](../architecture/git-aware-passive.md).
It is not implemented in the current MVP CLI.

`sqlrs diff` reports **what changed in the prepare context** between two inputs:
either two Git refs (e.g. base vs PR) or two local paths. Unlike plain `git diff`,
it focuses on the **set of migration/script files** that would be used by `sqlrs run`
with `--prepare <path>`, and produces a structured report (Added / Modified / Removed)
with optional content snippets and a short summary.

Use cases:

- Review migration changes between branches or tags before running them.
- Compare two local migration trees without Git.
- CI: emit a machine-readable diff for approval or notifications.

---

## Terminology

- **Prepare context** — the set of files (migrations, scripts) under a path that
  would be passed to `sqlrs run --prepare <path>` (or used with `--ref` in the
  future). The same notion is used to define “from” and “to” for the diff.
- **Changeset** — a single file-level change: Added, Modified, or Removed.

---

## Command Syntax

Two modes are mutually exclusive: **by refs** (Git) or **by paths** (local only).

### Mode 1: Diff by Git refs and prepare context

```text
sqlrs diff --from-ref <refA> --to-ref <refB> --prepare <path> [OPTIONS]
```

- `<refA>`, `<refB>` — Git refs: `HEAD`, `origin/main`, commit hash, tag (e.g. `v1.2.3`),
  or `refs/pull/123/head` if available locally.
- `<path>` — Path to migrations/scripts **relative to the repository root** (file or
  directory). Defines which files participate in the diff (same semantics as
  `sqlrs run --prepare` in the future).

### Mode 2: Diff two local sets (no Git)

```text
sqlrs diff --from-path <pathA> --to-path <pathB> [OPTIONS]
```

- `<pathA>`, `<pathB>` — Local paths (files or directories) to compare.

You must not mix refs and paths: e.g. `--from-ref` and `--from-path` cannot both
be set.

---

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json`. |
| `--include-content` | `false` | Include content snippets for Modified (and optionally Added/Removed) in the report. |
| `--limit` | (none) | Maximum number of change entries to list (for long diffs). |
| `--ref-mode` | `blob` | (Ref mode only.) How to read files from Git: `blob` (read from Git objects only) or `worktree` (temporary worktree). Same semantics as in `sqlrs run --ref` (see git-aware-passive). |
| `--workspace` | (config/cwd) | Workspace directory (repo root, etc.), same as for `sqlrs run`. |

---

## Output

### Text format (`--format text`)

- A short header with the compared contexts (from/to refs or paths).
- Sections: **Added**, **Modified**, **Removed**, each listing affected paths (and
  optionally identifiers such as hash/size).
- For **Modified**, with `--include-content`: unified-style snippets or short
  fragments.
- A **summary** line: counts of added, modified, and removed files (and optionally
  lines).
- Optionally a **DB impact** line: `yes` | `no` | `unknown` (heuristic: e.g. only
  comments/whitespace → `no`; otherwise `unknown` or `yes` if migration semantics
  can be inferred).

### JSON format (`--format json`)

A single JSON object with stable structure for scripting and CI:

- `from`, `to` — identifiers of the “from” and “to” contexts (ref or path).
- `added` — array of entries (path and optional content/hash).
- `modified` — array of entries (path, optional old/new hash, optional content).
- `removed` — array of entries (path and optional content).
- `summary` — object with counts (e.g. `added_count`, `modified_count`, `removed_count`).
- `db_impact` — optional string: `yes` | `no` | `unknown`.

When `--limit` is set, the arrays may be truncated; the summary still reflects
total counts when known.

---

## Validation and Errors

- **Not a git repository** — When using `--from-ref`/`--to-ref`, the current
  workspace must be inside a Git repo. Otherwise the CLI exits with an error and
  suggests using `--from-path`/`--to-path`.
- **Invalid or unresolved ref** — If a ref does not resolve to a commit/tree, the
  command fails with a clear message.
- **Conflicting mode** — If both ref-based and path-based options are given (e.g.
  `--from-ref` and `--from-path`), the command fails.
- **Missing path** — In path mode, both `--from-path` and `--to-path` must exist
  (file or directory).

---

## Examples

### Diff between branch and base (ref mode)

```bash
sqlrs diff --from-ref origin/main --to-ref HEAD --prepare migrations/
```

### Diff between two tags

```bash
sqlrs diff --from-ref v1.0 --to-ref v2.0 --prepare db/scripts
```

### JSON output with content and limit

```bash
sqlrs diff --from-ref main --to-ref feature/schema --prepare migrations/ \
  --format json --include-content --limit 50
```

### Diff two local directories (no Git)

```bash
sqlrs diff --from-path ./migrations-v1 --to-path ./migrations-v2
```

```bash
sqlrs diff --from-path ./old.sql --to-path ./new.sql --format json
```

---

## Implementation Notes (for implementers)

Algorithm (from design):

1. Load context files from `from-ref`/`to-ref` or `from-path`/`to-path` (using blob
   or worktree as per `--ref-mode` when in ref mode).
2. Normalize the input set (e.g. directory → ordered set of files).
3. Compute hashes and compare: Added/Removed by path, Modified by hash.
4. Optionally build a semantic hint (e.g. comments/whitespace only → low or no
   DB impact).
5. Emit the report in the requested format.

This command does not start an engine, run migrations, or touch the cache; it only
compares file sets and content.
