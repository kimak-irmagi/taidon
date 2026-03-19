# 2026-03-20 Alias Inspection CLI Shape

- Conversation timestamp: 2026-03-20T00:00:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: bounded scan scope for alias inspection

### Question discussed

How should `sqlrs alias` inspection commands choose the filesystem area they
scan for alias files?

### Alternatives considered

1. Always scan the entire active workspace recursively.
2. Always scan from the caller's current working directory recursively.
3. Allow an explicit scan root and bounded depth, with one default.

### Chosen solution

Adopt option 3.

`sqlrs alias ls` and `sqlrs alias check` in scan mode accept:

- `--from <workspace|cwd|path>`
- `--depth <self|children|recursive>`

The default scan mode is:

- `--from cwd`
- `--depth recursive`

The resolved scan root must stay within the active workspace.

### Brief rationale

Alias execution is already current-working-directory-relative, so inspection
should feel local-first by default in nested repository layouts. At the same
time, explicit scan-root and scan-depth controls keep workspace-wide inventory
possible without forcing it as the default for every invocation.

## Decision Record 2: inspection surface is `ls` plus `check`, without `show`

### Question discussed

Should the alias-inspection slice ship `sqlrs alias ls/show/validate`, or a
smaller surface?

### Alternatives considered

1. Keep `sqlrs alias ls/show/validate`.
2. Ship `sqlrs alias ls/check` and omit `show` from the current slice.
3. Collapse everything into one `alias inspect` command with multiple modes.

### Chosen solution

Adopt option 2.

The accepted alias-inspection surface is:

- `sqlrs alias ls [--prepare] [--run] [--from <workspace|cwd|path>] [--depth <self|children|recursive>]`
- `sqlrs alias check [--prepare] [--run] [--from <workspace|cwd|path>] [--depth <self|children|recursive>] [<ref>]`

`show` is intentionally omitted from this slice.

### Brief rationale

Alias files are already designed to remain small and human-readable. A semantic
`show` command would add little value at this stage compared with direct file
viewing, while `ls` and `check` cover the actual inventory and static
validation needs for repo-tracked recipes.

## Decision Record 3: rename `validate` to `check`

### Question discussed

What should the static-validation subcommand be called?

### Alternatives considered

1. Keep `validate`.
2. Rename it to `check`.
3. Use `verify`.

### Chosen solution

Adopt option 2.

Use `sqlrs alias check`.

`check` supports:

- scan mode when no `<ref>` is provided
- single-alias mode when `<ref>` is provided

### Brief rationale

`check` is shorter, reads more naturally as a subcommand, and still clearly
signals static verification rather than runtime execution.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/architecture/alias-inspection-flow.md`
- `docs/roadmap.md`
- `docs/adr/2026-03-18-mixed-alias-composite-grammar.md`

## Contradiction check

This ADR supersedes the alias-inspection command shape described in:

- [`2026-03-18-mixed-alias-composite-grammar.md`](2026-03-18-mixed-alias-composite-grammar.md)

The earlier ADR remains current for mixed raw/alias `prepare ... run`
composition and standalone/composite `run <run-ref>` semantics, but its
`sqlrs alias ls/show/validate` wording is no longer the source of truth.
