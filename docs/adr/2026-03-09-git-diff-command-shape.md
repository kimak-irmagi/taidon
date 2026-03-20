# 2026-03-09 Git Diff Command Shape

Status: Obsolete. Superseded by
[`2026-03-18-mixed-alias-composite-grammar.md`](2026-03-18-mixed-alias-composite-grammar.md).

- Conversation timestamp: 2026-03-09T10:13:04+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Obsolete

## Question discussed

How should `sqlrs diff` relate syntactically to the existing `plan`, `prepare`,
and `run` command families so that git-aware diffing stays close to the main
CLI contract?

## Alternatives considered

1. Keep `sqlrs diff --from-ref ... --to-ref ... --prepare <path>` as a
   standalone diff-specific DSL centered on a prepare path.
2. Make `sqlrs diff` a meta-command that wraps one existing content-aware sqlrs
   command (`plan:*`, `prepare:*`, later maybe `run:*`) and evaluates it in two
   contexts.
3. Support nested composite invocations immediately, such as
   `sqlrs diff ... prepare ... run ...`.

## Chosen solution

Adopt alternative 2.

`sqlrs diff` defines only the comparison scope (`from/to ref` or
`from/to path`) and then wraps exactly one existing sqlrs command using its
current syntax. The accepted initial scope is:

- wrapped commands: `plan:*`, `prepare:*`
- future extension: `run:*` only for file-backed inputs
- no nested composite `prepare ... run` in the first slice
- global `-v` remains the verbose flag
- global `--output` remains the human/json output selector

## Brief rationale

This keeps `sqlrs diff` syntactically aligned with the commands users already
know, avoids inventing a parallel mini-DSL around `--prepare <path>`, and makes
revision-dependent file discovery part of the wrapped command semantics rather
than an external side channel.

It also keeps the first implementation slice bounded: parsing and evaluating one
wrapped command is substantially simpler and clearer than supporting immediate
nested composites.

## Related documents

- `docs/user-guides/sqlrs-diff.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/git-aware-passive.md`

## Contradiction check

This ADR is obsolete.

It originally constrained `sqlrs diff` to exactly one wrapped command and
explicitly excluded wrapped `prepare ... run` composites from the first slice.
That restriction was later replaced by bounded parity with the main CLI:
`diff` now accepts either one wrapped command or one normal two-stage
`prepare ... run` composite, including mixed raw/alias stages. The current
source of truth is
[`2026-03-18-mixed-alias-composite-grammar.md`](2026-03-18-mixed-alias-composite-grammar.md).
