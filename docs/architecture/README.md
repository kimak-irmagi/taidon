# Architecture Documents

Entry points for Taidon architecture and service design.

- [`mvp-architecture.md`](mvp-architecture.md) — MVP service composition and goals.
- [`diagrams.md`](diagrams.md) — high-level component and k8s topology diagrams.
- [`local-deployment-architecture.md`](local-deployment-architecture.md) — local
  profile (thin CLI, ephemeral engine).
- [`shared-deployment-architecture.md`](shared-deployment-architecture.md) —
  team/cloud profile (gateway/orchestrator, multi-tenant).
- [`engine-internals.md`](engine-internals.md) — internal structure of the sqlrs
  engine.
- [`runtime-snapshotting.md`](runtime-snapshotting.md) — runtime storage model,
  snapshot/backends (OverlayFS/copy/etc).
- [`state-cache-design.md`](state-cache-design.md) — cache keys, triggers,
  retention, local store layout.
- [`state-cache-capacity-control.md`](state-cache-capacity-control.md) -
  bounded cache policy (capacity limits, watermarks, eviction semantics).
- [`sql-runner-api.md`](sql-runner-api.md) — runner API surface, timeouts,
  cancel, streaming.
- [`liquibase-integration.md`](liquibase-integration.md) - Liquibase provider
  strategy and structured logs.
- [`k8s-architecture.md`](k8s-architecture.md) - Kubernetes architecture with a
  single gateway entry point.
- [`cli-contract.md`](cli-contract.md) - CLI contract and commands.
- [`cli-architecture.md`](cli-architecture.md) - CLI flows for local vs remote
  and source uploads.
- [`cli-component-structure.md`](cli-component-structure.md) - CLI internal
  component structure.
- [`local-engine-component-structure.md`](local-engine-component-structure.md) -
  local engine component structure.
- [`prepare-job-events-flow.md`](prepare-job-events-flow.md) - events-first
  monitoring flow for prepare jobs.
- [`prepare-job-events-component-structure.md`][pjecs] -
  component structure for prepare events streaming and watch controls.
- [`prepare-manager-refactor.md`](prepare-manager-refactor.md) - prepare manager
  split into coordinator/executor/snapshot roles.
- [`statefs-component-structure.md`](statefs-component-structure.md) - StateFS
  contract and filesystem isolation in the engine.
- [`local-engine-storage-schema.md`](local-engine-storage-schema.md) - SQLite
  schema for local engine state.
- [`release-happy-path-e2e.md`](release-happy-path-e2e.md) - release-gated
  happy-path E2E flow and component structure.
- [`../api-guides/sqlrs-engine.openapi.yaml`][openAPI] -
  OpenAPI 3.1 spec for the local engine (MVP).
- [`query-analysis-workflow-review.md`](query-analysis-workflow-review.md) - notes
  on query analysis workflow.
- [`git-aware-passive.md`](git-aware-passive.md) — notes on the `git` interaction
  scenarios, initiated
  by user via the local CLI commands
- [`git-aware-active.md`](git-aware-active.md) — notes on the github interaction
  scenarios that require
  a running service/API endpoint

[pjecs]: prepare-job-events-component-structure.md
[openAPI]: ../api-guides/sqlrs-engine.openapi.yaml
