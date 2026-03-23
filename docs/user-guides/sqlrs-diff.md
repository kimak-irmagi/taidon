# sqlrs diff

Russian: [sqlrs-diff.RU.md](sqlrs-diff.RU.md)

## Overview

**Status: first slice implemented** in `frontend/cli-go` (`internal/diff`,
`internal/cli`, `internal/app`). Diff runs **without contacting the engine**
and compares **file-list closures** (paths + content hashes) built locally.

Broader behaviour is part of the git-aware passive set in
[`docs/architecture/git-aware-passive.md`](../architecture/git-aware-passive.md).

`sqlrs diff` is a **meta-command**: it wraps one existing content-aware sqlrs
command line, evaluates it in two filesystem contexts, and reports Added /
Modified / Removed files in the resolved input graph. The goal is to stay
syntactically close to the main sqlrs command surface rather than introducing a
separate `diff`-specific input DSL.

**What the first slice compares.** For `plan:psql`, `prepare:psql`, `plan:lb`, and
`prepare:lb`, the implementation uses the **same file-closure builders** on both
sides (psql `\i`/`\include` closure, Liquibase changelog graph). It does **not**
yet fetch engine task plans or prepare payloads, so “plan vs prepare” does not
change the diff output today—only the wrapped **kind** (psql vs lb) does.

**Design target (later slices).** Compatibility with composite shapes and richer
semantics:

| Form | Design target (future engine-backed / multi-phase) |
|------|----------------------------------------------------|
| `sqlrs diff ... plan ...` | Difference in **task plans** (ordered steps, task hashes, cacheability). |
| `sqlrs diff ... prepare ...` | Difference in **prepare** inputs / normalized task bodies. |
| `sqlrs diff ... run ...` | **File-backed** run inputs only. |

The same command line you use for `plan` / `prepare` / `run` is intended to be
reused with only the diff scope inserted once those slices land.

**Composite diff commands (design).** To maximise compatibility with the main CLI,
`diff` is ultimately a **group of composite commands**: you insert the diff scope
between `sqlrs` and the verb you would use for a real run. **Today only a single
wrapped command token** is accepted (`plan:psql`, `plan:lb`, `prepare:psql`,
`prepare:lb`). Shapes such as `prepare <alias> run:psql ...` are **not** parsed yet
(see [Composite `prepare ... run`](#composite-prepare--run) below).

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
- `<sqlrs-command>` is one wrapped sqlrs command and must use the **exact same
  syntax** as in the main CLI: same subcommand, same options, same `--` and
  trailing args. See [`sqlrs-plan.md`](sqlrs-plan.md), [`sqlrs-prepare.md`](sqlrs-prepare.md),
  and [`sqlrs-run.md`](sqlrs-run.md) (and their variant docs) for the authoritative
  syntax of each command.
- **Global options** apply as in any sqlrs invocation: `-v` / `--verbose` enables
  more verbose logging to stdout/stderr during the diff process; `--output human|json`
  selects the output format (no diff-specific `--format`).

Examples of wrapped commands **supported today**:

- `plan:psql`
- `plan:lb`
- `prepare:psql`
- `prepare:lb`

**Not yet:** `prepare <alias>`, `prepare … run …` composites, `run:*` (and any
invocation whose first token after the scope is not one of the four above).

Initial non-goals:

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
- Each ref is checked out as a **detached worktree** at the repository root
  (`git worktree add --detach`; cleaned up after the command unless
  `--ref-keep-worktree`). Paths from the wrapped command (e.g. `-f`, changelog)
  resolve relative to the **same logical cwd inside that worktree**. For example,
  if `sqlrs diff` is started from `<repo>/examples`, then `-f ./chinook/prepare.sql`
  is resolved against `<worktree>/examples/chinook/prepare.sql`. The Git repo is
  found from the process current working directory.
- **`--ref-mode blob`** is reserved; the CLI currently supports **`worktree` only**
  (default). Passing `blob` returns a clear “not supported” error.

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

**Revision-sensitive vs other arguments:** Only inputs that depend on the
repository or path context (e.g. files named by `-f`, or the include graph) are
compared. Arguments that do not depend on revision (e.g. `-c 'SELECT 1'` in psql
commands, or `--image postgres:17`) are the same on both sides; diff does not
invent semantics for them. For commands that mix file-backed and inline inputs,
only the file-derived part is diffed; the implementation may warn or reject
when there is nothing revision-dependent to compare.

### `plan:*` (first slice)

Compare the **resolved file graph** used as input to planning (same closure as for
`prepare:*` of the same kind). Engine **task plan** diff (steps, cache keys) is
not implemented yet.

### `prepare:*` (first slice)

Compare **resolved input files** and the expanded include/changelog graph
(content-addressed). Normalized prepare **task bodies** from the engine are not
compared yet.

### `run:*` (future / limited)

`run:*` is a future extension and is only meaningful for **file-backed inputs**.
Arguments that do not depend on revision (e.g. `-c 'select 1'` in `run:psql`) are
not diffable — the Git ref does not change that payload. Inline-only invocations
may be rejected or produce an empty diff. The same principle applies to other
kinds: only inputs that come from files (or from the resolved file graph) are
compared across refs.

### Composite `prepare ... run`

**Not supported in the first CLI slice.** The parser accepts exactly one wrapped
command token after the scope (e.g. `plan:psql`), not `prepare … run …` composites
or `prepare <alias>` alias invocations.

**Design** (for when this is implemented): the same normal two-stage composite
shape as the main CLI:

- `prepare <prepare-ref> run <run-ref>`
- `prepare <prepare-ref> run:<kind> ...`
- `prepare:<kind> ... run <run-ref>`
- `prepare:<kind> ... run:<kind> ...`

Rules (target):

- each phase resolved in the left/right context per main CLI rules;
- alias-backed stages resolve `*.prep.s9s.yaml` / `*.run.s9s.yaml` per side;
- diff reports **prepare** and **run** separately in output.

For scope modes, the effective context root is:

- **path mode**: `--from-path` / `--to-path` as given;
- **ref mode**: repository root at that revision inside each temporary worktree
  (not a subdirectory of your current cwd unless you adjust `-f` paths
  accordingly).

### Revision-dependent file discovery

The **set of files** that participate in the wrapped command can differ between
the two sides. For example, in the old revision a script may `\i` two files, and
in the new revision it may include three; or an included file may have been
renamed or removed. The diff implementation must perform **full file discovery
and include expansion in each side’s context** and report:

- added or removed files in the resolved graph,
- changes in the content of files that appear in both graphs.

So “what files matter” is derived per revision, not assumed to be the same on both
sides.

### File list construction per kind (core requirement)

From diff's perspective, the commands (plan, prepare, run) differ mainly in **how
the file list is built** for that kind. The implementation must be able to
construct the **closure of file inputs** for each prepare kind and run kind, using
the same rules as the main CLI.

| Kind | Entry point | Closure rule |
|------|-------------|--------------|
| **prepare:psql** / plan:psql | `-f <file>` (file path required; **not** `-f -` / stdin) | Closure over `\i`, `\ir`, `\include`, `\include_relative` (and any other include directives the engine expands). Start from the file(s) named in `-f`; plain `\i` / `\include` resolve from the wrapped command cwd, while `\ir` / `\include_relative` resolve from the including file directory; recursively add every file referenced by those directives. |
| **prepare:lb** / plan:lb | `--changelog-file <path>` | Closure over the changelog graph: start from the changelog file; add every file referenced by it (include, includeAll, etc.). Liquibase defines the graph; diff reuses the same resolution so the file set is identical to what prepare/plan would use. |
| **run:psql** (future) | `-f <file>` (file-backed only) | Same as prepare:psql: closure over `\i` / `\include` from each `-f` entry. Inline `-c` does not contribute to the file list. |
| **run:*** (other kinds, future) | Kind-specific | Each run kind defines which arguments are file-backed; the file list is built from those (e.g. script path, config path) and their kind-specific includes. |

Once the file list (closure) is built for each side, diff compares the two sets
and their contents (Added / Modified / Removed). Plan vs prepare vs run then
only affect *what* is derived from those files (task plan, task bodies, or run
payload); the mechanism for building the file list per kind is the main
implementation requirement.

---

## Diff-Specific Options

| Option | Description |
|--------|-------------|
| `--from-ref <ref>` | Left Git revision. |
| `--to-ref <ref>` | Right Git revision. |
| `--from-path <path>` | Left local context. |
| `--to-path <path>` | Right local context. |
| `--ref-mode worktree` | Ref mode only. **Implemented:** `worktree` (default). **`blob` is not implemented** (passing it errors). |
| `--ref-keep-worktree` | Ref mode only. Do not remove temporary worktrees after exit (debugging). |
| `--include-content` | Include content snippets in human/json output. |
| `--limit <n>` | Truncate listed entries for very large diffs. |

Output and verbosity are **global**, not diff-specific:

- `--output human|json` is the global output selector (same as for `ls`, `plan`,
  etc.). Use it to choose human-readable text or JSON.
- `-v` / `--verbose` is the global verbose flag: when set, sqlrs logs more detail
  about the diff process (e.g. scope resolution, file discovery) to stdout/stderr.

---

## Output

### Human output (`--output human`)

- Header: compared scope and wrapped command.
- Added / Modified / Removed sections.
- Optional content snippets when `--include-content` is set.
- Short summary counts.

**Future:** optional hints such as `db_impact`; separate **prepare** / **run**
sections when composite wrapping is implemented.

### JSON output (`--output json`)

A single JSON object with stable top-level fields:

- `scope` (includes `mode`, path or ref fields, `ref_mode`, `ref_keep_worktree` when relevant)
- `command`
- `added`, `modified`, `removed` (entries with `path`, `hash`, optional `content` when `--include-content`)
- `summary` (`added` / `modified` / `removed` counts)

When `--limit` is used, listed arrays may be truncated while summary counts
reflect the full diff.

**Future:** optional `db_impact`; nested `prepare` / `run` objects for composite
wrappers.

---

## Validation and Errors

- **Missing or conflicting scope** — exactly one of ref mode or path mode must be
  selected.
- **Invalid wrapped command** — `diff` requires one wrapped sqlrs command.
- **Unsupported wrapped command** — only `plan:psql`, `plan:lb`,
  `prepare:psql`, `prepare:lb` (e.g. `run:psql`, alias `prepare …`, and
  `prepare … run …` composites are rejected today).
- **Ref resolution failure** — one of the refs does not resolve locally.
- **Missing file at one ref** — the primary file (`-f`, changelog, etc.) must be
  present in **both** worktrees; otherwise diff fails with a filesystem error on
  the missing side.
- **Not a Git repository** — ref mode requires a Git repository context.
- **No revision-dependent payload** — for future `run:*` support, inline-only
  inputs may be rejected as non-diffable.

---

## Examples

### Minimal demo (two commits, same `-f` path)

Ref mode materializes each revision in a temporary Git worktree. The path you pass
to `-f` / `--changelog-file` must exist **on both** refs, or file stat will fail
(for example `from-ref: ...` in ref mode or `from-path: ...` in path mode).

To see a working ref diff without touching your real project, run from the repo root:

```bash
./scripts/sqlrs-diff/demo-diff-refs.sh
```

That script creates a throwaway repository, makes two commits that change `schema/a.sql`,
and runs:

```bash
sqlrs diff --from-ref HEAD~1 --to-ref HEAD plan:psql -- -f schema/a.sql
```

You should see **Modified: schema/a.sql** in human output and a non-empty `modified`
array in JSON.

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

### Mixed composite invocation (not supported yet)

```bash
# Design target only — current CLI rejects this shape (single wrapped token required).
# sqlrs diff --from-ref origin/main --to-ref HEAD prepare chinook run:psql -- -f ./queries.sql
```

---

## Implementation Notes

The core requirement is **building the file list (closure) for each prepare kind
and run kind** as defined in the table above; diff then compares those lists and
their contents. The rest is shared machinery.

1. Parse global flags first, exactly as in a normal sqlrs invocation.
2. Parse the `diff` scope block.
3. Parse the wrapped command using the **same grammar and validation rules** as
   the main CLI (so that the syntax stays compatible), including the normal
   two-stage `prepare ... run` composite forms.
4. For each side (from-ref/to-ref or from-path/to-path), build the **file list**
   for the wrapped kind: resolve ref or path, then apply that kind's closure rule
   (e.g. `\i`/`\include` for psql, changelog graph for lb). The file set and
   content can differ per side.
5. **First slice:** compare the two file lists only (hashes / optional content
   snippets). **Future:** engine-backed plan/prepare/run payloads and per-phase
   composite output.
6. Classify Added / Modified / Removed.
7. Render human or JSON output according to the global `--output` flag.

**Implemented layout:** `internal/app` dispatches `diff` and calls
`diff.ParseDiffScope`; `internal/cli.RunDiff` runs `diff.ResolveScope` (path or
ref worktrees), `BuildPsqlFileList` / `BuildLbFileList`, `Compare`, and renderers.

This command does not start an engine or execute SQL; it compares resolved file
sets and their contents. Compatibility with global `-v` and `--output` matches
other sqlrs commands.

For **component structure and call flow** (who builds file lists, who calls whom),
see [`docs/architecture/diff-component-structure.md`](../architecture/diff-component-structure.md).
