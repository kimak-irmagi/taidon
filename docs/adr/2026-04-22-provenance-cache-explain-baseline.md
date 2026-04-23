# 2026-04-22 Provenance and Cache-Explain Baseline

- Conversation timestamp: 2026-04-22T00:00:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: provenance surface for the first repository-aware baseline

### Question discussed

How should the first public provenance slice be exposed for repository-aware
local workflows after bounded local `plan` / `prepare --ref` is already
accepted?

### Alternatives considered

1. Add a separate `sqlrs provenance ...` command family that reconstructs or
   prints provenance independently from the main execution commands.
2. Add provenance as an additive `--provenance-path <path>` option on
   single-stage local `plan` / `prepare`, and write one JSON side artifact
   without changing the main stdout/stderr contract.
3. Extend `run` and composite `prepare ... run ...` in the first provenance
   slice so every local execution shape gets provenance at once.

### Chosen solution

Adopt option 2.

The accepted baseline is:

```text
sqlrs plan [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--provenance-path <path>] <prepare-ref>
sqlrs plan:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--provenance-path <path>] ...

sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--provenance-path <path>] <prepare-ref>
sqlrs prepare:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] [--provenance-path <path>] ...
```

This slice stays bounded to:

- single-stage local `plan` / `prepare`;
- raw and alias-backed prepare flows;
- normal local filesystem and bounded local `--ref`;
- JSON side artifacts only.

It does not yet add:

- standalone `run` provenance;
- composite `prepare ... run ...` provenance;
- automatic provenance emission;
- provenance printed as part of the main command result.

### Brief rationale

`--provenance-path` is the smallest additive surface that makes real local
prepare-oriented workflows reproducible without redesigning existing human/JSON
result envelopes. Keeping the first slice on single-stage `plan` / `prepare`
reuses the already accepted bounded ref scope and avoids pulling composite run
semantics into the same PR.

## Decision Record 2: cache-explain surface and engine/API boundary

### Question discussed

How should sqlrs expose user-facing cache-hit/miss explanation for one
repository-aware prepare-oriented decision without mixing it into store-health
diagnostics or normal plan output?

### Alternatives considered

1. Extend `sqlrs status --cache` or `sqlrs ls --states --cache-details` with an
   extra mode for one-off invocation explanation.
2. Add explanation details directly into `sqlrs plan` output and treat cache
   reasoning as part of the normal planning surface.
3. Add a separate read-only command `sqlrs cache explain prepare ...` and back
   it with a dedicated read-only engine endpoint that computes the same final
   prepare signature and cache lookup as real execution.

### Chosen solution

Adopt option 3.

The accepted baseline is:

```text
sqlrs cache explain prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] <prepare-ref>
sqlrs cache explain prepare:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--image <image-id>] [--] [tool-args...]
```

with a read-only engine API:

```text
POST /v1/cache/explain/prepare
```

The first slice is limited to one single-stage prepare-oriented decision and
does not yet support:

- wrapped `plan`;
- wrapped `run`;
- composite `prepare ... run ...`;
- remote/server-side explanation.

`sqlrs status --cache` and `sqlrs ls --states --cache-details` remain the
operator/store-health surfaces.

### Brief rationale

`cache explain` answers a different question from store-health diagnostics and
from plan rendering: not "is cache healthy?" and not "what will run?", but
"will this exact prepare-oriented invocation hit cache, and why?". A dedicated
read-only command and endpoint keep that diagnostic narrow, scriptable, and
aligned with the real engine cache lookup instead of approximating it in the
CLI.

## Decision Record 3: internal owner for the shared bound trace

### Question discussed

Where should the first shared provenance/cache-explain trace assembly live on
the CLI side once both features need the same bound prepare metadata, input
manifest, and engine explain response?

### Alternatives considered

1. Create a new top-level CLI package dedicated to provenance/cache diagnostics
   immediately.
2. Keep the shared trace helper package-local inside `internal/app` while it is
   still bounded to single-stage local prepare-oriented commands.
3. Duplicate small provenance and cache-explain builders independently inside
   each command handler.

### Chosen solution

Adopt option 2.

The accepted baseline keeps a package-local prepare-trace helper in
`internal/app` that:

- reuses existing raw/alias/ref stage binding;
- reuses `internal/inputset` for deterministic input collection and hashing;
- calls the read-only cache-explain API;
- feeds both provenance writing and cache-explain rendering.

This helper does not replace the existing ownership boundaries:

- `internal/refctx` still owns ref-backed filesystem context;
- `internal/alias` still owns alias resolution/loading;
- `internal/inputset` still owns per-kind file semantics;
- `internal/cli` still owns rendering.

### Brief rationale

At this stage the shared trace is still an orchestration concern tightly coupled
to single-stage local `plan` / `prepare`. A new top-level package would add
boundary weight before the domain is broad enough to justify it, while
duplicating the trace would recreate the same drift that the shared inputset and
ref-context work just removed.

## Related documents

- `docs/user-guides/sqlrs-provenance.md`
- `docs/user-guides/sqlrs-cache-explain.md`
- `docs/architecture/provenance-cache-flow.md`
- `docs/architecture/provenance-cache-component-structure.md`
- `docs/architecture/cli-contract.md`
- `docs/api-guides/sqlrs-engine.openapi.yaml`

## Contradiction check

No existing ADR was marked obsolete.

This ADR complements earlier accepted decisions:

- `2026-04-08-bounded-local-ref-cli-shape.md` still defines the bounded local
  `--ref` scope that provenance and cache explanation build upon;
- `2026-03-22-shared-inputset-layer.md` still defines the shared source of
  truth for file-bearing semantics now reused by provenance and cache explain;
- `2026-03-09-bounded-cache-diagnostics-surface.md` still governs store-health
  diagnostics via `status --cache` and `ls --states --cache-details`.

This ADR only adds a separate per-invocation explanation surface and does not
replace those operator-facing diagnostics.
