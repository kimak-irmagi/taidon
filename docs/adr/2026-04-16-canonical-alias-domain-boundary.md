# 2026-04-16 Canonical Alias Domain Boundary

- Conversation timestamp: 2026-04-16T18:49:28.2119926+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

When cleaning up duplicated alias ownership between `frontend/cli-go/internal/app`
and `frontend/cli-go/internal/alias`, what should be moved first into the alias
package so that execution and inspection share one canonical alias definition
model without unnecessarily widening the refactor?

## Alternatives considered

1. Keep the current split and continue letting `internal/app` and
   `internal/alias` maintain separate prepare/run YAML loaders and schema rules.
2. Move alias definition loading, normalization, and schema validation into
   `internal/alias` first, but leave execution-specific path-resolution wrappers
   in `internal/app` for now.
3. Move both alias definition loading and all execution-path resolution into one
   shared `internal/alias` API in the same PR.

## Chosen solution

Adopt option 2.

`PR2` introduces one canonical execution-facing alias definition owner inside
`internal/alias`.

That owner must provide:

- one shared loaded alias-definition model for prepare and run aliases
- one shared YAML loader and schema validator used by both execution and
  inspection
- filesystem-aware loading so ref-backed prepare alias execution can load alias
  files from a supplied filesystem context

`internal/app` remains responsible in this PR for:

- alias invocation argument parsing
- command-specific execution flow
- any path-resolution wrappers that are still needed to preserve current
  command-specific user-facing errors

## Brief rationale

The highest-value duplication today is not path derivation itself; it is that
execution and inspection own separate YAML structs, kind normalization, and
schema validation rules for the same alias files. Removing that duplication
first gives `internal/alias` clear domain ownership while keeping the patch
small enough to avoid accidental CLI-facing regressions in error wording or
ref-backed execution flow.

Moving full execution-path resolution in the same step would enlarge the change
surface and conflate two different concerns: canonical alias definition
ownership and CLI-facing path-selection behavior.

## Related documents

- `docs/architecture/cli-maintainability-refactor.md`
- `docs/architecture/alias-inspection-component-structure.md`
- `docs/architecture/alias-create-component-structure.md`

## Contradiction check

No existing ADR was marked obsolete.

This decision narrows the next alias maintainability step without changing the
accepted file-based alias storage model, alias invocation grammar, or alias
path-resolution semantics recorded in earlier ADRs.
