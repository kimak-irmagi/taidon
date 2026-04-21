# 2026-04-20 Plan/Prepare Stage Pipeline Shape

- Conversation timestamp: 2026-04-20T09:35:20.0434251+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: where to place the shared plan/prepare pipeline boundary

### Question discussed

For the CLI maintainability `PR4`, how should `sqlrs plan` and `sqlrs prepare`
share one internal pipeline without changing the public CLI contract or
overreaching into transport/runtime layers?

### Alternatives considered

1. Keep the existing split and only extract a few helper functions from
   `plan.go` and `prepare.go`.
2. Move the shared plan/prepare pipeline down into `internal/cli` so transport
   execution and app-level orchestration are unified together.
3. Introduce one shared package-local stage pipeline in `internal/app`, keep
   kind-specific binding in `ref_prepare.go`, and keep terminal transport
   behavior in `internal/cli`.

### Chosen solution

Adopt option 3.

`PR4` is scoped to:

- one shared package-local stage pipeline in `frontend/cli-go/internal/app`;
- one shared request/runtime model for `plan` and `prepare` stage execution;
- shared image resolution, bind orchestration, and cleanup handling;
- mode-specific terminal actions only for:
  - `plan` result waiting and rendering;
  - `prepare` submit/watch handling and DSN/job-ref rendering;
- reuse of the same pipeline by direct and alias-backed `plan` / `prepare`
  paths.

Explicitly out of scope:

- changing CLI syntax, output shape, watch semantics, or exit codes;
- moving the transport contract out of `frontend/cli-go/internal/cli`;
- redesigning `internal/refctx` or alias path resolution;
- folding composite `run` orchestration into the same pipeline.

### Brief rationale

The current duplication sits in CLI-side orchestration, not in the transport
layer. A small helper-only pass would keep the ownership blurry, while pushing
the boundary into `internal/cli` would mix app-level command semantics with
runtime execution. Keeping the shared pipeline in `internal/app` removes the
duplicated parse/bind/config/invoke/cleanup flow while preserving the existing
ownership of ref-aware binding and transport behavior.

## Related documents

- `docs/architecture/cli-maintainability-refactor.md`
- `docs/architecture/cli-component-structure.md`
- `docs/architecture/ref-component-structure.md`
- `docs/adr/2026-04-16-cli-maintainability-pr-sequencing.md`

## Contradiction check

No existing ADR was marked obsolete.

This ADR refines the accepted `PR4` step from
`docs/adr/2026-04-16-cli-maintainability-pr-sequencing.md`; it does not replace
that sequencing decision.
