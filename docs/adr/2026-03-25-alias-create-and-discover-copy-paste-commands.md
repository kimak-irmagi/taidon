# 2026-03-25 Alias Create Command and Discover Copy-Paste Output

- Conversation timestamp: 2026-03-25T11:52:32.9892645+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

Should `sqlrs discover --aliases` mutate the workspace with an `--apply`-style
flag, or should alias creation remain a separate write command with discover
printing a copy-pasteable command?

## Alternatives considered

1. Add `discover --apply` and let discovery write suggested alias files.
2. Keep discovery read-only and show only an informational alias suggestion.
3. Keep discovery read-only, add explicit `sqlrs alias create`, and have
   discovery print a ready-to-run `sqlrs alias create ...` command for each
   strong candidate.

## Chosen solution

Adopt option 3.

`discover --aliases` stays advisory and read-only. It ranks candidate workflow
entrypoints and prints a copy-pasteable `sqlrs alias create ...` command for
each strong suggestion. The actual write path lives in the separate
`sqlrs alias create` command.

## Brief rationale

This keeps command responsibilities clean:

- discovery remains analysis-only and does not mutate files;
- alias creation is explicit, auditable, and easy to filter or script;
- the user gets an immediately usable command instead of an abstract hint;
- the command surface stays smaller than a generic `discover --apply` design,
  which would need extra filtering, confirmation, and conflict handling flags.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/architecture/discover-flow.md`
- `docs/architecture/discover-component-structure.md`
- `docs/architecture/alias-create-flow.md`
- `docs/architecture/alias-create-component-structure.md`
- `docs/adr/2026-03-25-discover-alias-analyzer-shape.md`
- `docs/adr/2026-03-16-file-based-aliases-and-discover-command.md`

## Contradiction check

No direct contradictions were found in existing ADRs.

This ADR refines the discover decision by separating the analysis step from the
workspace write step. It preserves the earlier decision that `discover` is an
advisory verb and adds the explicit mutating alias creation command as the
actionable follow-up.
