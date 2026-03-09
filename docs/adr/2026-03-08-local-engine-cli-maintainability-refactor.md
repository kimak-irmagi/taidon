# 2026-03-08 Maintainability Refactor Boundaries for Local Engine and CLI

- Conversation timestamp: 2026-03-08T00:38:51.7532295+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should the next refactoring pass reduce maintainability risk in the local
engine and CLI without changing the external CLI contract, HTTP API, or storage
schema?

## Alternatives considered

1. Keep the current structure and rely on the existing test suite.
2. Perform a large rewrite into new packages and broader architectural changes.
3. Perform an incremental boundary cleanup inside the current modules and
   packages, starting with HTTP route extraction and CLI invocation context
   cleanup, then continuing with narrower prepare collaborators.

## Chosen solution

Use the incremental boundary-cleanup approach documented in
`docs/architecture/local-engine-cli-maintainability-refactor.md`.

The accepted direction is:

- keep public CLI commands, endpoint paths, and storage contracts unchanged;
- split `internal/httpapi` by resource-oriented route registration and shared
  response helpers;
- introduce a single CLI invocation context and option builders in
  `frontend/cli-go/internal/app`;
- continue narrowing `internal/prepare` by extracting internal roles for job
  journaling, plan building, and cache/snapshot policy instead of keeping more
  low-level helper ownership inside large orchestration files;
- keep the work incremental and test-gated by phase.

## Brief rationale

The current codebase is functionally healthy and heavily tested, but core files
such as `internal/prepare/manager.go`, `internal/prepare/execution.go`,
`internal/httpapi/httpapi.go`, `internal/app/app.go`, `internal/app/init_wsl.go`,
and `internal/cli/commands_prepare.go` concentrate too many responsibilities.

A large rewrite would increase delivery risk and blur behavioral regression
boundaries. Keeping the current structure would continue increasing coupling and
review cost. Incremental boundary cleanup preserves existing behavior while
making later changes smaller, more reviewable, and easier to test.

## Related documents

- `docs/architecture/local-engine-cli-maintainability-refactor.md`
- `docs/architecture/prepare-manager-refactor.md`
- `docs/architecture/local-engine-component-structure.md`
- `docs/architecture/cli-component-structure.md`

## Contradiction check

No existing ADR was marked obsolete. This decision is additive to earlier
prepare-component split decisions and defines the next maintainability-oriented
refactoring pass.
