# 2026-04-07 Discover Generic Analyzers Component Structure

- Conversation timestamp: 2026-04-07T13:29:28.1193957+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should the generic `sqlrs discover` slice be split internally so that:

- multiple analyzers can coexist without `internal/app` turning into a bag of
  heuristics;
- workflow analyzers and repository-hygiene analyzers can share one report
  surface;
- follow-up command rendering stays read-only and shell-aware without leaking
  file-mutation logic into every analyzer?

## Alternatives considered

1. Keep one growing aliases-oriented implementation and add generic analyzer
   branches directly inside it.
2. Introduce a registry-driven `internal/discover` package with per-analyzer
   implementations, shared report types, and a dedicated follow-up-command
   renderer.
3. Push analyzer-specific logic back into `internal/app` and keep
   `internal/discover` as only an aliases-specific helper.

## Chosen solution

Adopt option 2.

The generic slice uses:

- a registry-driven `internal/discover` orchestrator;
- one analyzer implementation file per stable analyzer;
- shared report and finding types across analyzers;
- dedicated follow-up command rendering inside `internal/discover`, using shell
  family from command context;
- continued reuse of `internal/alias` for alias coverage and `internal/inputset`
  for workflow-oriented closure semantics.

## Brief rationale

This split keeps discovery semantics cohesive without coupling them to CLI
parsing or to alias-specific code paths.

- The registry keeps analyzer selection additive and extensible.
- Per-analyzer files keep repository-hygiene and workflow-shaping logic from
  collapsing into one monolith.
- A dedicated follow-up renderer centralizes shell-aware command formatting and
  avoids duplicating PowerShell/POSIX rules across analyzers.
- The design preserves the read-only contract: analyzers describe and suggest,
  but mutation still happens only if the user manually runs a printed command.

## Related documents

- `docs/user-guides/sqlrs-discover.md`
- `docs/architecture/discover-flow.md`
- `docs/architecture/discover-component-structure.md`
- `docs/architecture/cli-contract.md`
- `docs/adr/2026-04-07-discover-generic-analyzers-cli-shape.md`

## Contradiction check

No direct contradictions were found in accepted ADRs.

This ADR narrows the internal structure for the already accepted generic
discover CLI shape. Earlier obsolete ADRs about the initial aliases-only
discover slice remain obsolete and do not need further updates.
