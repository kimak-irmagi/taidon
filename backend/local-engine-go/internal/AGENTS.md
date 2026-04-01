# Agent notes for backend local-engine internals

- When adding or changing a prepare/run kind, treat it as a cross-cutting change.
- Update the engine surface in the same change:
  - `internal/prepare` task planning, execution, and validation for the kind.
  - `internal/run` execution plumbing and result handling for the kind.
  - any request, response, queue, or persistence shape that the kind touches.
- Keep the engine-side kind set aligned with `frontend/cli-go/internal`.
- Do not land a new kind in only one layer of the system.
