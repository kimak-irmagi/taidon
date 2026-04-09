# 2026-04-08 Bounded Local Ref CLI Shape

- Conversation timestamp: 2026-04-08T14:00:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: which commands receive the first local `--ref` slice

### Question discussed

After `sqlrs diff` ref mode is already available, should the next public
Git-aware slice add `--ref` to all repository-aware command shapes at once, or
should it stay bounded to a smaller command set first?

### Alternatives considered

1. Add `--ref` to `plan`, `prepare`, `run`, and mixed `prepare ... run ...`
   composites in one slice.
2. Add `--ref` only to single-stage `plan` and `prepare`, for both raw and
   alias-backed prepare flows.
3. Skip `plan` / `prepare` and start with `run --ref` as the main public entry
   point.

### Chosen solution

Adopt option 2.

The accepted bounded local CLI shape is:

```text
sqlrs plan [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] <prepare-ref>
sqlrs plan:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] ...

sqlrs prepare [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] <prepare-ref>
sqlrs prepare:<kind> [--ref <git-ref>] [--ref-mode worktree|blob] [--ref-keep-worktree] [--watch|--no-watch] ...
```

This slice does not yet add:

- standalone `run --ref`
- `prepare ... run ...` with a ref-backed prepare stage
- provenance or `cache explain`

### Brief rationale

`plan` and `prepare` are the smallest revision-sensitive execution surfaces that
already own deterministic input binding. Extending them first gives public user
value without simultaneously introducing second-stage run semantics, per-stage
revision propagation rules, or provenance surface changes in the same PR.

## Decision Record 2: path base inside a selected ref

### Question discussed

When `--ref` is used, should alias refs and raw file-bearing paths resolve from
the repository root, or from the caller's current working directory projected
into the selected revision?

### Alternatives considered

1. Rebase everything to the repository root when `--ref` is present.
2. Preserve current-working-directory-relative semantics by projecting the
   caller cwd into the selected ref.
3. Use repository-root-relative semantics for alias mode and projected-cwd
   semantics for raw mode.

### Chosen solution

Adopt option 2.

Under `--ref`, sqlrs:

- finds the repository root from the caller cwd;
- resolves the selected ref locally;
- projects the caller cwd into that ref context;
- resolves alias refs and raw file-bearing paths from that projected cwd.

### Brief rationale

This keeps `plan` / `prepare` under `--ref` aligned with today's local path-base
rules and with the already accepted `sqlrs diff` ref-context model. Rebasing to
the repository root would make ref-backed execution behave differently from both
normal execution and ref-backed diff for the same relative path arguments.

## Related documents

- `docs/user-guides/sqlrs-ref.md`
- `docs/architecture/ref-flow.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/architecture/git-aware-passive.md`

## Contradiction check

Earlier accepted ADRs about `sqlrs diff` ref-mode defaults still apply:

- `worktree` remains the default ref mode;
- `--ref-keep-worktree` remains valid only with `worktree`.

This ADR narrows only the next public `plan` / `prepare` slice and does not
change those previously accepted defaults.
