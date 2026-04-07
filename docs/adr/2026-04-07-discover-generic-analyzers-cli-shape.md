# 2026-04-07 Discover Generic Analyzer CLI Shape

- Conversation timestamp: 2026-04-07T13:29:28.1193957+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: bare `discover` default behavior

### Question discussed

Once generic analyzers are added, should bare `sqlrs discover` continue to act
like `discover --aliases`, or should it run the full stable analyzer set?

### Alternatives considered

1. Keep bare `discover` as an aliases-only shortcut forever.
2. Change bare `discover` to run all stable analyzers in canonical order once
   the generic analyzer slice ships.
3. Require at least one explicit analyzer flag and make bare `discover` a usage
   error.

### Chosen solution

Adopt option 2.

The accepted CLI shape is:

```text
sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]
```

Analyzer flags are additive. If no analyzer flags are provided, `discover` runs
all stable analyzers in canonical order. If one or more analyzer flags are
provided, `discover` runs exactly that subset.

### Brief rationale

Bare `discover` should become the one-shot advisory pass for a repository once
multiple analyzers exist. Keeping it aliases-only would leave the command
surprisingly narrow and would force users to learn a longer flag list for the
common "show me everything relevant" case.

## Decision Record 2: follow-up command output for hygiene analyzers

### Question discussed

Should `discover --gitignore` and `discover --vscode` only print advisory text,
or should they also emit copy-pasteable follow-up commands the way
`discover --aliases` emits `sqlrs alias create ...`?

### Alternatives considered

1. Keep `--gitignore` and `--vscode` text-only and reserve commands for a later
   mutating workflow.
2. Emit copy-pasteable follow-up commands while keeping `discover` itself
   read-only.
3. Add `--apply`/`--fix` modes directly into the first generic analyzer slice.

### Chosen solution

Adopt option 2.

`discover --gitignore` and `discover --vscode` remain advisory and read-only,
but each finding may include a ready-to-copy follow-up command:

- `--gitignore`: shell-native command for appending missing ignore entries;
- `--vscode`: shell-native command for creating or merging the missing
  `.vscode/*.json` entries.

When shell syntax matters, follow-up commands are rendered for the current shell
family:

- PowerShell on Windows shells;
- POSIX shell otherwise.

For existing `.vscode/*.json` files, the suggested command must merge only the
missing sqlrs-related entries and preserve unrelated user settings.

### Brief rationale

This preserves the current read-only contract of `discover` while still making
the findings actionable. It gives `--gitignore` and `--vscode` the same
copy-paste ergonomics that already make `discover --aliases` useful, without
adding a mutating command mode that would expand the review surface of the
first generic slice.

## Related documents

- `docs/user-guides/sqlrs-discover.md`
- `docs/architecture/discover-flow.md`
- `docs/architecture/discover-component-structure.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/m2-local-developer-experience-plan.md`

## Contradiction check

No direct contradictions were found in accepted ADRs.

This ADR extends the previously accepted aliases analyzer work with the generic
analyzer CLI behavior and follow-up command model. Older obsolete ADRs that
mentioned reserved discover flags remain obsolete and do not need further
changes.
