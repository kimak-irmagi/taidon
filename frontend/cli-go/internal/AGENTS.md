# Agent notes for frontend CLI internals

- When adding or changing a prepare/run kind, treat it as a cross-cutting change.
- Update the CLI surface in the same change:
  - `internal/cli` command parsing, help text, and usage examples.
  - `internal/alias` create/check/list behavior and target path conventions.
  - `internal/diff` wrapped-command validation and result formatting.
  - `internal/discover` alias discovery, ranking, and `discover --aliases` output.
  - `internal/inputset/*` collectors and validators for the kind.
- Keep the supported-kind story aligned with `backend/local-engine-go/internal`.
- Do not land a new kind in only one of these layers.
