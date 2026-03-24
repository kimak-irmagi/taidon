# 2026-03-23 Psql Include Base Semantics

- Conversation timestamp: 2026-03-23T07:50:07.6493999+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

Should the shared `psql` file-set collector resolve plain `\i` / `\include`
from the first `-f` file directory, or from the effective command working
directory, while keeping `\ir` / `\include_relative` script-relative?

## Alternatives considered

1. Resolve all include forms from the first `-f` file directory.
2. Resolve all include forms from the effective command working directory.
3. Follow PostgreSQL `psql` semantics: plain `\i` / `\include` resolve from
   the effective command working directory, while `\ir` / `\include_relative`
   resolve from the directory of the including file.

## Chosen solution

Adopt option 3.

The shared CLI-side `psql` inputset collector resolves plain includes from the
effective command working directory and resolves relative includes from the
including file directory.

Diff-side worktree/ref resolution still defines the effective working directory
on each side; the collector does not substitute the first `-f` file directory
for plain includes.

## Brief rationale

This matches `psql` semantics and keeps `sqlrs` compatible with migration of
existing scripts. The temporary first-file-directory shortcut was useful for a
CI workaround, but it taught a non-psql rule and created surprising results for
users who already rely on `\i` being cwd-relative.

Scripts that need sibling-relative includes should use `\ir` /
`\include_relative`. Scripts that use plain `\i` / `\include` keep the normal
command-cwd behavior users expect from `psql`.

## Related documents

- `docs/architecture/inputset-component-structure.md`
- `docs/user-guides/sqlrs-prepare-psql.md`
- `docs/user-guides/sqlrs-diff.md`
- `docs/adr/2026-03-22-shared-inputset-layer.md`

## Contradiction check

No direct contradictions were found in existing ADRs.
