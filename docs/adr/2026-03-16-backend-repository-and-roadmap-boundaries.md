# 2026-03-16 Backend Repository and Roadmap Boundaries for Shared Engine

- Conversation timestamp: 2026-03-16T14:30:29.7638885+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should Taidon continue the transition from local/public work toward the
closed Team/Shared backend with respect to:

- repository topology for closed components;
- when and how the current `SQL-Runner-API` repository should be moved;
- where roadmap material should live once part of it becomes non-public.

## Alternatives considered

1. Keep Team/Shared backend components in a group of separate private
   repositories from the start.
2. Move all current work, including the current `SQL-Runner-API` repository,
   into a private monorepo immediately.
3. Use a private backend monorepo for closed Team/Shared components, keep the
   public/local work in the main public repository for now, and move the
   current `SQL-Runner-API` repository only after its target role in the shared
   backend is fixed.

## Chosen solution

Use option 3.

The accepted direction is:

- keep the main public `taidon` repository as the public-facing product and
  documentation perimeter for local/open work;
- use a private backend monorepo for closed Team/Shared backend components,
  aligned with the existing backend structure (`gateway`, `libs`, `services`);
- do not move the current `SQL-Runner-API` repository immediately;
- move that repository only after its target architectural role is fixed, and
  import it with history into the appropriate location in the private backend
  monorepo;
- split roadmap ownership:
  - public roadmap stays in the public repository and covers public product
    direction and open deliverables;
  - closed Team/Shared sequencing, service decomposition, rollout planning, and
    internal milestones live in private planning/docs together with the private
    backend monorepo.

## Brief rationale

The architecture documents already point toward a shared backend with a common
control plane and service-oriented backend structure rather than a set of
independently evolving repositories. In particular:

- `docs/requirements-architecture.md` defines a single invariant core with
  separate deployment profiles;
- `docs/architecture/shared-deployment-architecture.md` describes a shared
  service topology with gateway, orchestrator, runner, cache, artifact, and
  control-plane concerns;
- `docs/architecture/local-engine-component-structure.md` keeps the local
  engine as a distinct deployment unit with its own internal boundaries;
- the current `backend/` layout in the main repository already reserves
  `gateway`, `libs`, and `services/*` as the natural backbone for a monorepo.

Starting with multiple private repositories would introduce early versioning and
cross-repository coordination costs before service boundaries are truly stable.
Moving `SQL-Runner-API` immediately would freeze placement before its role is
fully decided and would likely import temporary compatibility work into the
private backend too early.

The roadmap must be split because the public roadmap should continue to express
product direction and open milestones, while Team/Shared internal sequencing and
implementation plans will contain non-public information.

## Related documents

- `docs/requirements-architecture.md`
- `docs/architecture/shared-deployment-architecture.md`
- `docs/architecture/local-engine-component-structure.md`
- `docs/roadmap.md`

## Contradiction check

No existing ADR was marked obsolete. This decision is additive and defines
repository and roadmap governance for the next Team/Shared phase.
