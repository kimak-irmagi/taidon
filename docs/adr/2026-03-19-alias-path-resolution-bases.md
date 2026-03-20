# 2026-03-19 Alias Path Resolution Bases

- Conversation timestamp: 2026-03-19T09:06:49.5026281+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: base path for alias refs

### Question discussed

Should sqlrs resolve `plan <ref>`, `prepare <ref>`, and `run <ref>` alias refs
relative to the workspace root, or relative to the caller's current working
directory?

### Alternatives considered

1. Keep alias refs workspace-root relative once a workspace is known.
2. Resolve alias refs relative to the caller's current working directory while
   still enforcing workspace boundaries.
3. Introduce a separate alias namespace or hidden alias tree so that relative
   filesystem semantics do not matter.

### Chosen solution

Adopt option 2.

Alias refs are current-working-directory-relative logical stems:

- `sqlrs prepare chinook` -> `<cwd>/chinook.prep.s9s.yaml`
- `sqlrs run smoke` -> `<cwd>/smoke.run.s9s.yaml`
- exact-file escape via a trailing `.` also stays current-working-directory
  relative after stripping the final dot

Workspace discovery still matters, but only as a boundary and config root:

- the resolved alias file must remain within the active workspace;
- sqlrs must not silently reinterpret alias refs as workspace-root relative.

### Brief rationale

This matches user expectations in nested repository layouts and avoids forcing
users to `cd` to the workspace root or to restate longer composed paths. A
command such as `cd examples/chinook && sqlrs prepare ../chinook` should work
without special casing.

## Decision Record 2: base path for file-bearing arguments inside alias files

### Question discussed

When an alias file contains relative file-bearing arguments such as
`-f prepare.sql` or `--changelog-file changelog.xml`, what should those paths be
relative to?

### Alternatives considered

1. Resolve them relative to the caller's current working directory.
2. Resolve them relative to the workspace root.
3. Resolve them relative to the alias file directory.

### Chosen solution

Adopt option 3.

Relative file-bearing paths read from an alias file resolve relative to that
alias file's directory.

This rule applies only to data loaded from the alias file itself. Raw command
stages keep their existing path semantics:

- `prepare:psql -- -f queries.sql` keeps current-working-directory-relative
  behavior;
- `run:psql -- -f queries.sql` keeps current-working-directory-relative
  behavior.

### Brief rationale

This makes alias files self-contained and versionable together with the scripts
or changelogs they reference. Moving or reviewing an alias file does not
require implicit knowledge of the caller's working directory.

## Decision Record 3: `sqlrs diff` inherits the same path bases

### Question discussed

Should `sqlrs diff` use a different path-resolution model for alias-backed
stages, or should it inherit the same alias-ref and alias-file path bases as the
main CLI?

### Alternatives considered

1. Keep a separate diff-specific resolution model based on workspace roots.
2. Reuse the same path bases as the wrapped main-CLI command on each side of
   the diff.

### Chosen solution

Adopt option 2.

`sqlrs diff` inherits the same path semantics as the wrapped command:

- alias refs resolve from the effective working directory on each side;
- file-bearing paths read from an alias file resolve from that alias file's
  directory on that side.

For scope modes, the effective working directory is:

- in path mode, the side root selected by `--from-path` or `--to-path`;
- in ref mode, the caller's current working directory projected into each ref
  tree.

### Brief rationale

Diff should not introduce a second mental model for the same wrapped command.
Keeping path bases aligned with normal execution reduces surprise and preserves
parity between analysis and execution.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/user-guides/sqlrs-prepare.md`
- `docs/user-guides/sqlrs-plan.md`
- `docs/user-guides/sqlrs-run.md`
- `docs/user-guides/sqlrs-diff.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/m2-local-developer-experience-plan.md`

## Contradiction check

This ADR supersedes the path-resolution part of:

- [`2026-03-16-file-based-aliases-and-discover-command.md`](2026-03-16-file-based-aliases-and-discover-command.md)

That older ADR already remains obsolete for broader alias grammar reasons; its
workspace-root-relative alias-resolution text is no longer current.
