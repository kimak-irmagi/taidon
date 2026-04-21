# 2026-04-21 WSL Init PR5 Shape

- Conversation timestamp: 2026-04-21T14:08:40.8351806+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: how to shape the optional WSL/init maintainability follow-up

### Question discussed

After landing the CLI maintainability `PR1`-`PR4` slices, how should the
optional WSL/init follow-up reduce the size of `internal/app/init_wsl.go`
without changing CLI behavior or creating another broad package split?

### Alternatives considered

1. Leave the current WSL/init helpers as-is and avoid a dedicated follow-up.
2. Extract WSL/init orchestration into a new exported or cross-package module.
3. Keep the work inside `frontend/cli-go/internal/app`, split WSL/init helpers
   by responsibility, and introduce one package-local dependency carrier for
   WSL/host execution seams.

### Chosen solution

Adopt option 3.

`PR5` is scoped to:

- splitting `init_wsl.go` into narrower package-local helpers for bootstrap,
  storage/mount orchestration, and command-execution utilities;
- moving shared WSL path/config helpers such as `resolveWSLSettings` and
  `windowsToWSLPath` out of `app.go` when they exist only for WSL runtime
  wiring;
- routing the WSL/host command seams through one package-local dependency
  carrier instead of multiplying direct package-global hooks across new files;
- preserving `sqlrs init local`, warning behavior, path translation behavior,
  cleanup/progress behavior, and exit-code semantics.

Explicitly out of scope:

- changing `sqlrs init` syntax, workspace config schema, or privilege model;
- redesigning Linux btrfs init or daemon/runtime contracts;
- introducing a new exported package boundary for WSL/init logic;
- revisiting runner, alias, discover, or shared stage-pipeline refactors.

### Brief rationale

The remaining WSL/init complexity is file-local and platform-heavy, not an
indicator that the code needs a new public package boundary. A package-local
split keeps ownership with `internal/app`, reduces review cost, and preserves
the established command/runtime seams while still making bootstrap, storage, and
mount flows independently reviewable and testable.

## Related documents

- `docs/architecture/cli-maintainability-refactor.md`
- `docs/architecture/cli-component-structure.md`
- `docs/architecture/local-engine-cli-maintainability-refactor.md`
- `docs/adr/2026-04-16-cli-maintainability-pr-sequencing.md`

## Contradiction check

No existing ADR was marked obsolete.

This ADR refines the accepted optional `PR5` step from
`docs/adr/2026-04-16-cli-maintainability-pr-sequencing.md` and aligns the older
broader direction in `docs/architecture/local-engine-cli-maintainability-refactor.md`;
it does not replace either document.
