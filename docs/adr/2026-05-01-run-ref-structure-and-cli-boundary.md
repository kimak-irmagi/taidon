# 2026-05-01 Run Ref Structure and CLI Boundary

- Conversation timestamp: 2026-05-01T14:02:25.1933968+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: API boundary for the first standalone `run --ref` slice

### Question discussed

Should the first bounded local `run --ref` slice introduce a new engine/API
surface for repository-aware run execution, or stay fully CLI-side and reuse
the existing run transport?

### Alternatives considered

1. Add a new run API or extend the engine request shape so the engine receives
   Git ref metadata and performs repository-backed file loading itself.
2. Keep the first slice CLI-only: resolve refs, aliases, and file-bearing
   inputs locally, then send the same materialized `cli.RunOptions` /
   `client.RunRequest` shape that standalone `run` already uses today.
3. Delay `run --ref` entirely until a larger hosted/server-side Git design is
   ready.

### Chosen solution

Adopt option 2.

The accepted first slice remains fully CLI-side:

- `sqlrs run --ref <git-ref> <run-ref> --instance <id|name>`
- `sqlrs run:psql --ref <git-ref> --instance <id|name> ...`
- `sqlrs run:pgbench --ref <git-ref> --instance <id|name> ...`

The CLI resolves the selected ref locally, binds raw or alias-backed inputs
through the shared ref/alias/inputset layers, materializes the normal
transport-ready run request, and then calls the existing run API unchanged.

No new engine endpoint or OpenAPI update is introduced in this slice.

### Brief rationale

The currently approved scope is local-only, standalone-only, and limited to
file-backed run inputs whose semantics already live in the CLI. Pushing Git ref
awareness into the engine now would enlarge the slice without adding user value
for the first PR.

## Decision Record 2: internal owner for ref-aware standalone run binding

### Question discussed

Once standalone `run --ref` is approved, where should the ref-aware binding
logic live inside the CLI?

### Alternatives considered

1. Reuse the prepare-oriented shared stage pipeline and pull standalone `run`
   into the same `stageRunRequest` / `stageRuntime` abstraction.
2. Create a new top-level CLI package dedicated to ref-backed run execution.
3. Keep one package-local run-binding helper inside `internal/app`, while
   preserving `internal/refctx`, `internal/alias`, `internal/inputset`, and
   `internal/cli` as the long-term owners of their existing domains.

### Chosen solution

Adopt option 3.

The accepted structure keeps:

- composite-shape rejection in `internal/app/runner` when the `run` stage
  carries `--ref`;
- run-specific parsing, instance arbitration, and orchestration in
  `internal/app`;
- one package-local run-binding helper in `internal/app` that:
  - resolves or borrows a ref context;
  - resolves run aliases against the selected filesystem view;
  - reuses shared `psql` / `pgbench` inputset projectors;
  - produces the normal `cli.RunOptions` payload plus cleanup;
- shared ownership boundaries unchanged elsewhere:
  - `internal/refctx` owns repo/ref/projected-cwd/worktree/blob context;
  - `internal/alias` owns alias-target resolution and YAML loading;
  - `internal/inputset` owns per-kind file semantics;
  - `internal/cli` owns run transport and streamed output handling.

### Brief rationale

Standalone `run` already converges in `internal/app` before calling the
existing transport layer, but its lifecycle still differs from the
prepare-oriented stage pipeline. A package-local binding helper keeps the first
slice small and consistent with current boundaries without creating another
premature top-level package.

## Related documents

- `docs/user-guides/sqlrs-run-ref.md`
- `docs/architecture/run-ref-flow.md`
- `docs/architecture/run-ref-component-structure.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/cli-component-structure.md`

## Contradiction check

No existing ADR was marked obsolete.

This ADR complements earlier accepted decisions:

- `2026-04-08-bounded-local-ref-cli-shape.md` still governs the original
  bounded local `plan` / `prepare --ref` scope and explicitly left standalone
  `run --ref` for a later follow-up.
- `2026-04-09-bounded-local-ref-component-structure.md` still governs shared
  `internal/refctx` ownership for ref-backed filesystem context and remains
  consistent with standalone `run --ref` consuming that shared layer.
- `2026-04-22-provenance-cache-explain-baseline.md` still governs the
  prepare-oriented diagnostics baseline and remains separate from run-side
  provenance or cache explanation.
