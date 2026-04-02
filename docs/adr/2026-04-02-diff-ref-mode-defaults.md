# 2026-04-02 Diff Ref Mode Defaults

- Conversation timestamp: 2026-04-02T14:15:10.9362579+07:00
- GitHub user id: @Bel9shik
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: default ref mode for `sqlrs diff`

### Question discussed

Should `sqlrs diff --from-ref ... --to-ref ...` default to Git-object `blob`
reads or to detached `worktree` checkouts for the current CLI slice?

### Alternatives considered

1. Default to `blob` reads via `internal/inputset.GitRevFileSystem` and require
   users to opt into `worktree` only when they need full filesystem semantics.
2. Default to `worktree` and keep `blob` as an explicit opt-in for lighter
   object-database reads.

### Chosen solution

Adopt option 2.

`sqlrs diff` keeps `worktree` as the default ref mode. Explicit
`--ref-mode blob` remains supported for object-database reads, but it is not the
default contract.

### Brief rationale

The current diff collectors and wrapped command semantics still rely on normal
filesystem behavior in cases such as symlink resolution. Making `blob` the
default before it matches those semantics creates user-visible regressions in
repositories that already work in `worktree` mode. Keeping `worktree` as the
default preserves compatibility while still allowing explicit lighter-weight
blob reads where appropriate.

## Decision Record 2: `--ref-keep-worktree` outside `worktree` mode

### Question discussed

When a user passes `--ref-keep-worktree` together with ref mode `blob`, should
the CLI ignore that flag or reject the combination?

### Alternatives considered

1. Ignore `--ref-keep-worktree` outside `worktree` mode and continue.
2. Reject the combination with a user-facing error stating that
   `--ref-keep-worktree` is only valid with `--ref-mode worktree`.

### Chosen solution

Adopt option 2.

Passing `--ref-keep-worktree` without `--ref-mode worktree` is a CLI error.

### Brief rationale

Silently ignoring flags makes the CLI harder to reason about and hides mistaken
user input. An explicit error keeps the command contract honest: the flag refers
to a resource that only exists in `worktree` mode, so using it elsewhere should
fail immediately.

## Related documents

- `docs/user-guides/sqlrs-diff.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/diff-component-structure.md`
- `docs/architecture/git-aware-passive.md`

## Contradiction check

No direct contradictions were found in existing ADRs.
