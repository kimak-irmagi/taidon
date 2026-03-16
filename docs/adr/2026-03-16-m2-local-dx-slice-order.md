# 2026-03-16 M2 Local Developer Experience Slice Order

- Conversation timestamp: 2026-03-16T16:20:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should the public/local M2 developer-experience work be split into
implementation slices so that:

- each slice delivers standalone user value;
- the public roadmap stays focused on open/local deliverables;
- git-aware features are introduced without coupling early slices to
  private/shared infrastructure or oversized command-surface changes?

## Alternatives considered

1. Start with the full `--ref` workflow first, then add supporting internals and
   diagnostics later.
2. Start with `sqlrs diff` as the first public git-aware command and design the
   required shared primitives along the way.
3. Start with local repository/workspace conventions, then add shared input
   graph primitives, then ship git-aware commands in bounded public slices.

## Chosen solution

Adopt option 3.

The approved slice order is:

1. repo/workspace conventions;
2. shared local input graph primitives;
3. `sqlrs diff` first public slice (`--from-path` / `--to-path`, wrapped
   `plan:*` and `prepare:*`);
4. git ref execution baseline (`--ref`, `blob|worktree`, bounded local scope);
5. provenance and `cache explain`.

The initial M2 plan explicitly excludes from the first public slices:

- hosted/shared Git integrations;
- PR automation;
- `sqlrs compare`;
- `run:*` support inside the first `sqlrs diff` slice;
- nested composite `prepare ... run` inside `sqlrs diff`.

## Brief rationale

This sequence creates the best balance between public value and implementation
risk.

Starting with repo/workspace conventions provides immediate local DX benefit
without forcing Git-aware plumbing into the first PR. Introducing shared local
input graph primitives next gives one deterministic base for `diff`, `--ref`,
provenance, and cache explanation. Shipping `sqlrs diff` in path mode before
Git-ref mode validates the CLI UX and comparison model while avoiding early
Git object-access complexity.

Only after those foundations are in place should the plan add `--ref`, and only
after repository-aware execution exists should the work add provenance and cache
explain as explanation layers.

This decision keeps the public roadmap focused on open/local work and avoids
pulling Team/Shared rollout concerns back into the M2 implementation plan.

## Related documents

- `docs/roadmap.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/architecture/git-aware-passive.md`
- `docs/architecture/diff-component-structure.md`
- `docs/adr/2026-03-09-git-diff-command-shape.md`

## Contradiction check

No existing ADR was marked obsolete.

This decision is additive. It does not change the accepted `sqlrs diff` command
shape from `2026-03-09-git-diff-command-shape.md`; it only fixes the ordering
and scope boundaries of the public/local M2 implementation slices.
