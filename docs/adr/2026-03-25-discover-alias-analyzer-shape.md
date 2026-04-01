# 2026-03-25 Discover Alias Analyzer Shape

- Conversation timestamp: 2026-03-25T09:24:46.6127627+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should `sqlrs discover --aliases` find useful alias candidates without
collapsing into a duplicate of `sqlrs alias ls`?

## Alternatives considered

1. Only enumerate existing alias files or simple alias-gap pairs.
2. Use a cheap prefilter, then deeper kind-specific validation and topology
   analysis to identify likely workflow entrypoints and suggest alias files.
3. Require explicit metadata or manual curation before discovery can report
   anything useful.

## Chosen solution

Adopt option 2.

The initial `discover --aliases` slice will:

- scan the workspace with cheap path and content heuristics first;
- use deeper kind-specific validation and closure collection only for promising
  candidates;
- build a topology over discovered files and rank likely start files as alias
  candidates;
- suppress or downgrade suggestions that are already covered by existing
  repo-tracked aliases.

The command remains advisory and read-only. In the current slice, bare
`discover` defaults to the aliases analyzer.

## Brief rationale

This is the first design that can surface real value for repository authors
without turning discovery into a duplicate inventory command.

- It finds likely entrypoint files, not only files that already look like
  aliases.
- It stays fast by filtering cheaply before it spends effort on closure
  analysis.
- It reuses the shared CLI-side semantics that already know how to validate and
  collect supported workflow kinds.
- It leaves room for later analyzers without locking the command to one naming
  heuristic.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/architecture/discover-flow.md`
- `docs/architecture/discover-component-structure.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/adr/2026-03-16-file-based-aliases-and-discover-command.md`
- `docs/adr/2026-03-22-shared-inputset-layer.md`

## Contradiction check

No direct contradictions were found in existing ADRs.

This ADR refines the broad `discover` decision by choosing the alias analyzer
shape and its candidate-ranking strategy. The umbrella command decision in
[`2026-03-16-file-based-aliases-and-discover-command.md`](2026-03-16-file-based-aliases-and-discover-command.md)
remains valid.
