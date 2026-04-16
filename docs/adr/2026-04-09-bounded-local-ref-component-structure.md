# 2026-04-09 Bounded Local Ref Component Structure

- Conversation timestamp: 2026-04-09T12:30:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: where ref-backed filesystem context lives

### Question discussed

Once bounded local `plan` / `prepare --ref` is added after `sqlrs diff` ref
mode already exists, where should repository-root resolution, projected-cwd
mapping, detached-worktree lifecycle, and blob-backed filesystem setup live?

### Alternatives considered

1. Keep ref-backed context setup inside `internal/diff` and let `plan` /
   `prepare` call into diff-owned helpers.
2. Duplicate the needed ref-resolution and worktree/blob logic directly inside
   plan/prepare orchestration.
3. Extract one shared CLI-side owner for ref-backed filesystem context and let
   both `diff` and `plan` / `prepare` consume it.

### Chosen solution

Adopt option 3.

The approved structure introduces a shared `internal/refctx` package that owns:

- repository-root discovery from caller cwd
- local Git-ref resolution
- projected-cwd resolution inside the selected revision
- detached-worktree creation and cleanup
- blob-backed filesystem setup for Git-object reads

`internal/diff` keeps scope parsing, comparison, and rendering.

### Brief rationale

Ref-backed filesystem context is not a diff-specific concern. Keeping it inside
`internal/diff` would create the wrong dependency direction for execution
commands. Reimplementing it for `plan` / `prepare` would duplicate behavior and
increase the risk of diverging projected-cwd or cleanup semantics.

## Decision Record 2: where alias loading happens under `--ref`

### Question discussed

When alias-backed `plan` / `prepare` runs under `--ref`, should alias loading be
reimplemented in a new ref-specific binder, or should existing alias ownership
be preserved?

### Alternatives considered

1. Add a separate ref-specific alias loader outside `internal/alias`.
2. Keep `internal/alias` as the source of truth and make it filesystem-aware so
   it can load alias files from a supplied ref-backed context.
3. Push alias loading down into `internal/inputset`.

### Chosen solution

Adopt option 2.

`internal/alias` remains the source of truth for:

- suffix conventions
- exact-file escape handling
- YAML loading
- alias schema validation

It must become capable of resolving and loading alias files against a supplied
filesystem context rather than assuming the live host filesystem only.

### Brief rationale

Alias-file semantics are still alias semantics, not inputset semantics. Keeping
ownership in `internal/alias` avoids splitting one concept across two modules
while still allowing ref-backed loading.

## Related documents

- `docs/user-guides/sqlrs-ref.md`
- `docs/architecture/ref-flow.md`
- `docs/architecture/ref-component-structure.md`
- `docs/architecture/diff-component-structure.md`
- `docs/architecture/cli-component-structure.md`

## Contradiction check

The earlier accepted `sqlrs diff` component structure remains valid after one
adjustment: generic ref-backed filesystem setup is no longer a long-term
exclusive responsibility of `internal/diff`; it moves to shared `internal/refctx`.
