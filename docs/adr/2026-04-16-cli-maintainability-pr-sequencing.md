# 2026-04-16 CLI Maintainability PR Sequencing

- Conversation timestamp: 2026-04-16T17:00:49.8493337+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: how to stage the next CLI maintainability refactor

### Question discussed

After several feature-by-feature CLI slices, what should be the next
maintainability pass, and how should it be staged to reduce technical debt
without changing the public CLI contract?

### Alternatives considered

1. Keep shipping features and avoid a dedicated refactoring pass.
2. Perform one large CLI rewrite that addresses `internal/app`,
   `internal/alias`, `internal/discover`, and `plan` / `prepare` duplication in
   one step.
3. Use a staged CLI-only roadmap with narrow PRs that each remove one concrete
   source of coupling.

### Chosen solution

Adopt option 3.

The accepted sequence is:

- `PR1`: introduce an explicit runner boundary in `frontend/cli-go/internal/app`
- `PR2`: centralize alias loading and validation under one canonical owner in
  `frontend/cli-go/internal/alias`
- `PR3`: separate generic `discover` report contracts from the aliases analyzer
- `PR4`: unify duplicated `plan` / `prepare` execution stages behind one shared
  pipeline
- `PR5` optional: split platform-heavy files such as WSL-specific init helpers
  after the higher-value boundaries are in place

### Brief rationale

The current CLI remains behaviorally healthy, but responsibility is now spread
across oversized files and partially duplicated domains. A big rewrite would
increase review and regression risk, while continuing feature-only work would
keep raising the cost of each next change. A staged plan preserves delivery
velocity while making each boundary cleanup reviewable and testable on its own.

## Decision Record 2: what the first PR should change

### Question discussed

Which maintainability improvement should be implemented first so that later CLI
refactors have a narrower and safer starting point?

### Alternatives considered

1. Start by centralizing alias domain ownership.
2. Start by unifying `plan` / `prepare` internals.
3. Start by introducing a top-level runner boundary in `internal/app` and
   replacing top-level package-state dispatch hooks with explicit dependencies.

### Chosen solution

Adopt option 3.

`PR1` is scoped to:

- making `app.Run(args)` a thin facade over a runner with explicit orchestration
  dependencies
- removing package-level mutable hooks from top-level dispatch seams
- keeping public CLI syntax, outputs, and exit-code behavior unchanged
- leaving deeper command-specific helpers and lower-level dependency seams out
  of scope for this PR

### Brief rationale

`internal/app` is the current coupling hotspot. It owns parse/dispatch flow,
writers, command-context setup, and composite cleanup orchestration, while tests
patch package globals to steer top-level behavior. Introducing one explicit
runner boundary reduces package-state coupling immediately and creates a stable
base for the later alias, discover, and stage-pipeline refactors.

## Related documents

- `docs/architecture/cli-maintainability-refactor.md`
- `docs/architecture/cli-component-structure.md`
- `docs/architecture/discover-component-structure.md`
- `docs/architecture/ref-component-structure.md`

## Contradiction check

No existing ADR was marked obsolete.

This decision refines the CLI-specific next step after
`docs/adr/2026-03-08-local-engine-cli-maintainability-refactor.md`; it does not
replace that broader engine-and-CLI direction.
