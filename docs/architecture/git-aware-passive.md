# Git-aware semantics: passive features (CLI)

Status: **proposed future design**. Flags like `--ref` and `--prepare` are not
available in the current MVP CLI. Today, the MVP uses a composite invocation
such as `sqlrs prepare:psql ... run:psql ...`.

Goal: add git-aware capabilities **without changing the user's work habits**. All functions in this document are activated **only by explicit user commands/flags** and do not require repository setup "for Taidon".

## Design principles

- **Do not guess intent from the repo.** The user defines context: `--prepare <path>` (migration file/dir) and the `sqlrs run -- <cmd>` command.
- **Minimum side effects.** By default we do not run `git checkout`, do not touch the working tree, and avoid extra temp files. `worktree` mode is an explicit opt-in and leaves minimal traces under `.git/worktrees` that can be cleaned up.
- **Fast path first.** Try to find a ready state in the Taidon cache by hashes of the involved files. If not found, build it.
- **Everything is reproducible.** Any execution can save a manifest (provenance) to reproduce the same state 1:1.
- **Remote mode requires repo access.** For `--ref` on a remote runner, the service must have a server-side mirror or VCS secrets; otherwise the CLI uploads sources to `source storage` and passes `source_id` (see [`sql-runner-api.md`](sql-runner-api.md)).

---

## Scenario P1. Run by git ref without checkout: `--ref`

### Motivation

User wants to bring up a state **as in commit/branch/tag** without touching the current working directory (dirty state, open IDEs, parallel tasks).

### UX / CLI

Base pattern stays the same, we add a single flag.

```bash
sqlrs run --dbms postgres:17 \
  --workspace ./sqlrs-work \
  --ref <git-ref> \
  --prepare <path> \
  -- psql -c "select 1"
```

Where `<git-ref>` can be: `HEAD`, `origin/main`, `abc1234`, `v1.2.3`, `refs/pull/123/head` (if available locally).

Important for remote runner: `--ref` works only if the service has access to the repo (server-side mirror or VCS secrets for clone/read). Otherwise the CLI must upload sources to `source storage` and pass `source_id` (see [`sql-runner-api.md`](sql-runner-api.md)).

Behavior options:

- `--ref-mode blob|worktree` (default `blob`)
  - `blob`: read needed files directly from git objects (no full checkout)
  - `worktree`: create a temporary `git worktree` and remove it after run
- `--ref-keep-worktree` (debugging: do not remove the temporary worktree)

### Implementation sketch

1. Detect repo root (if missing, error `not a git repo`).
2. Resolve `<git-ref>` to `commit/tree`.
3. Get file list under `--prepare` and their blob hashes:
   - blob mode: `git ls-tree -r <ref> -- <path>` (no checkout)
   - worktree mode: `git worktree add --detach <tmpdir> <ref>`
4. Compute file hashes from `blob OID` (optionally include key deps: configs, include files).
5. Build the change chain and query Taidon cache.
6. On cache hit, return/use the ready state.
7. On miss, read file contents (blob or worktree), run `prepare` (migrations/scripts) in Taidon, create snapshots.
8. Continue with `sqlrs run -- <cmd>` in the resulting environment.
9. Generate provenance (see P4) if enabled.

---

## Scenario P2. Zero-copy cache hit (speed-up without file extraction)

### Motivation

In large repositories, checkout is expensive, while Taidon may already have the required state by migration hashes.

### UX / CLI

Enabled automatically in `--ref-mode blob` (or via an explicit flag):

```bash
sqlrs run --ref <ref> --ref-mode blob --prepare migrations/ -- <cmd>
```

Optional:

- `--zero-copy=auto|off` disable optimization or keep auto mode

### Implementation

1. In blob mode, get file list and blob hashes via `git ls-tree`.
2. Compute cache keys from hashes.
3. Check Taidon cache **before** extracting files.
4. If cache hit, do not extract files to disk, return the environment immediately.

Note: if required blob objects are missing locally (`partial clone`/LFS), a `git fetch` is needed or switch to `worktree` mode.

---

## Scenario P3. Taidon-aware diff by wrapping an existing sqlrs command: `diff`

### Motivation

`git diff` shows text, but the user wants "what changed **from the perspective of
an actual sqlrs command**" while staying syntactically close to the main CLI.

### UX / CLI

Instead of inventing a separate `--prepare <path>` mini-DSL, `diff` wraps one
existing content-aware sqlrs command and evaluates it in two contexts.

```bash
sqlrs diff --from-ref <refA> --to-ref <refB> plan:psql -- -f ./prepare.sql
sqlrs diff --from-ref <refA> --to-ref <refB> prepare:lb -- update --changelog-file db/changelog.xml
sqlrs diff --from-path <pathA> --to-path <pathB> prepare:psql -- -f ./prepare.sql
```

Rules:

- diff-specific options come before the wrapped command;
- the wrapped command keeps its current syntax unchanged;
- global `-v` stays the verbose control and global `--output` stays the text/json selector;
- wrapped-command file-bearing semantics come from the shared CLI-side
  `inputset` component for the selected kind;
- **Current CLI slice** (`frontend/cli-go`): exactly **one** wrapped token among
  `plan:psql`, `plan:lb`, `prepare:psql`, `prepare:lb`; compares **file-list
  closures** (hashes) only—no engine. **Ref mode** defaults to **`blob`** reads
  (`git show` / `git ls-tree`); **`worktree`** remains available explicitly.
- **Design / later**: two-stage `prepare ... run` composites and alias-backed
  `prepare <ref>`; full derived representations (task plans, prepare payloads).
- Future standalone `run:*` support is possible only for file-backed inputs,
  because inline-only invocations may have no revision-dependent payload.

Examples of composite shapes (**design target**; not all parsed by the CLI yet):

```bash
sqlrs diff --from-ref <refA> --to-ref <refB> prepare chinook run:psql -- -f ./queries.sql
sqlrs diff --from-path <pathA> --to-path <pathB> prepare:psql -- -f ./prepare.sql run smoke
```

What `diff` compares:

- **Today:** same **resolved file graph + content** for `plan:*` and `prepare:*`
  of a given kind (psql vs lb).
- **Design target:** `plan:*` -> derived task plan; `prepare:*` -> bodies + graph;
  `run:*` -> file-backed inputs only.

### Implementation

1. Parse the diff scope (`from/to ref` or `from/to path`).
2. Parse the wrapped command using the same CLI grammar as the main invocation
   (today: single `plan:*` or `prepare:*` token only).
3. For each side independently, bind the wrapped command through the shared
   `inputset` component for that kind and collect the resulting file set using
   that side's own revision or path context.
4. **Today:** build file-list closures and compare Added / Modified / Removed.
   **Later:** richer representations and per-phase output for `prepare ... run`.
5. Render human or JSON.

---

## Scenario P4. Provenance (execution manifest)

### Motivation

In real commands you quickly need to answer "what exactly did you run?" A portable artifact helps reproduce issues months later.

### UX / CLI

Auto-enabled via flag or config:

```bash
sqlrs run --provenance write --provenance-path ./artifacts/provenance.json -- <cmd>
```

Modes:

- `write` - write file
- `print` - print summary to stdout
- `both`

Content (minimum):

- timestamp (start time)
- git ref + commit (if `--ref` used)
- `dirty/clean` (working tree state)
- input file list from `--prepare` + hashes
- environment params (`dbms.image`, key flags)
- Taidon snapshot chain used (base/derived)
- command `sqlrs run -- <cmd>` + argv

### Implementation

1. Collect run context at start.
2. During `prepare`, record snapshot chain and key decisions (cache hit/miss).
3. Serialize JSON (and optional text summary) on exit.

---

## Scenario P5. Compare: one query on two states

### Motivation

QA/dev needs to compare results/errors between two schema versions (e.g., base vs PR).

### UX / CLI

```bash
sqlrs compare \
  --from-ref <refA> --from-prepare <path> \
  --to-ref <refB> --to-prepare <path> \
  -- psql -c "select * from flights limit 10"
```

Output:

- exit codes
- stderr/stdout (with limits)
- (optional) diff of result sets in table format

Options:

- `--diff text|json|table`
- `--timeout-ms 5000`
- `--max-rows 1000`

### Implementation

1. Bring up environments for `from-ref` and `to-ref` (with cache).
2. Execute the same command in both.
3. Collect results and compare.

---

## Scenario P6. "Explain cache": why fast/slow

### Motivation

User wants to know why this run was slow: no snapshot? hashes changed? different engine?

### UX / CLI

```bash
sqlrs cache explain --ref <ref> --prepare <path>
```

Output:

- computed changeset hashes
- nearest anchor (if any)
- miss reason (no snapshot / engine+version mismatch / missing chain segment)

### Implementation

1. Compute the same key(s) as for `migrate/run`.
2. Query cache index.
3. Render explanation.

---

## Minimal MVP for passive features

1. `--ref` (blob mode) + zero-copy cache hit
2. `sqlrs diff --from-ref/--to-ref <wrapped-command...>` for one `plan:*` or `prepare:*` command
3. provenance (write)
4. `cache explain` (simple version)
