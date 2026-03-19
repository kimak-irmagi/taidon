# 2026-03-18 Mixed Alias/Raw Composite Grammar

- Conversation timestamp: 2026-03-18T16:45:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: mixed raw/alias `prepare ... run` composition

### Question discussed

How should sqlrs combine repo-tracked alias mode with the existing raw
`prepare:<kind>` / `run:<kind>` command families in a normal two-stage
`prepare ... run` invocation?

### Alternatives considered

1. Keep alias mode standalone only, and keep composite execution limited to raw
   `prepare:<kind> ... run:<kind> ...`.
2. Introduce a separate composite-specific syntax for aliases.
3. Allow the normal two-stage `prepare ... run` invocation to mix raw and alias
   stages freely.

### Chosen solution

Adopt option 3.

The accepted grammar is:

- `sqlrs prepare <prepare-ref> run <run-ref>`
- `sqlrs prepare <prepare-ref> run:<kind> ...`
- `sqlrs prepare:<kind> ... run <run-ref>`
- `sqlrs prepare:<kind> ... run:<kind> ...`

The composite remains bounded to exactly two stages:

- one `prepare` stage
- one `run` stage

Each stage may use raw mode or alias mode.

### Brief rationale

This preserves one intuitive composition model instead of splitting sqlrs into
“alias-only” and “raw-only” execution families. Users can adopt repo-tracked
aliases incrementally without losing the existing prepare/run pipeline shape.

## Decision Record 2: standalone and composite semantics for `sqlrs run <run-ref>`

### Question discussed

How should `sqlrs run <run-ref>` behave in standalone and composite invocations?

### Alternatives considered

1. Require `--instance` in all alias-mode `run` invocations, including when a
   preceding `prepare` already produced an instance.
2. Allow `run <run-ref>` to inherit the instance from a preceding `prepare`,
   while still allowing `--instance` in the same composite invocation.
3. Require `--instance` only in standalone alias-mode `run`, and forbid
   `--instance` when a preceding `prepare` already selects the instance.

### Chosen solution

Adopt option 3.

- Standalone `sqlrs run <run-ref>` requires `--instance <id|name>`.
- In `prepare ... run <run-ref>`, the `run` stage consumes the instance
  produced by the preceding `prepare`.
- In that composite form, `--instance` is forbidden and reported as an explicit
  ambiguity error.
- Alias stages keep their kind and tool args in the alias file; they do not
  accept per-stage inline tool arguments.

### Brief rationale

This keeps standalone run execution explicit while preserving the existing
mental model of composite `prepare ... run` as a pipeline from one prepared
instance into one run stage. Forbidding `--instance` in that composite avoids
silent precedence rules.

## Decision Record 3: diff grammar parity with main CLI

### Question discussed

Should `sqlrs diff` keep rejecting wrapped `prepare ... run` composites, or
should it accept the same bounded two-stage composite grammar as the main CLI?

### Alternatives considered

1. Keep `diff` limited to exactly one wrapped command in all cases.
2. Add the same bounded two-stage `prepare ... run` composite grammar to
   `diff`, while still rejecting longer command chains.

### Chosen solution

Adopt option 2.

`sqlrs diff` accepts:

- one wrapped `plan`, `prepare`, or future file-backed `run` command; or
- one normal two-stage `prepare ... run` composite using the same mixed raw and
  alias grammar as the main CLI.

For wrapped composites:

- the `prepare` phase and the `run` phase are resolved independently on each
  side of the diff scope;
- diff output reports those phases separately;
- longer command chains remain out of scope.

### Brief rationale

Once the main CLI accepts mixed raw/alias `prepare ... run` composition, making
`diff` reject that same bounded shape would create a confusing split-brain
grammar. Parity with the main CLI reduces surprise while still keeping the
implementation bounded to two phases.

## Decision Record 4: updated M2 execution slice

### Question discussed

How should the next public/local M2 slice be framed after the prepare-alias
baseline landed?

### Alternatives considered

1. Keep run aliases, alias inspection, and mixed prepare/run composition as
   separate follow-up slices.
2. Combine run aliases, alias inspection, and mixed prepare/run composition
   into the next PR slice.

### Chosen solution

Adopt option 2.

The next M2 slice after the prepare-alias baseline is:

- run aliases
- alias inspection (`sqlrs alias ls/show/validate`)
- mixed raw/alias `prepare ... run` composition

### Brief rationale

These changes define one coherent execution surface. Shipping them together
avoids a temporary state where `prepare` aliases exist but the corresponding
`run`/composite story remains artificially split.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/user-guides/sqlrs-prepare.md`
- `docs/user-guides/sqlrs-run.md`
- `docs/user-guides/sqlrs-diff.md`
- `docs/architecture/cli-contract.md`
- `docs/architecture/m2-local-developer-experience-plan.md`

## Contradiction check

This ADR supersedes:

- [`2026-03-09-git-diff-command-shape.md`](2026-03-09-git-diff-command-shape.md)
- [`2026-03-16-file-based-aliases-and-discover-command.md`](2026-03-16-file-based-aliases-and-discover-command.md)

Those ADRs should be marked obsolete and point here as the current source of
truth for mixed alias/raw composite grammar and diff parity.
