# Architecture Documents

Entry points for Taidon architecture and service design.

- `mvp-architecture.md` — MVP service composition and goals.
- `diagrams.md` — high-level component and k8s topology diagrams.
- `local-deployment-architecture.md` — local profile (thin CLI, ephemeral engine).
- `shared-deployment-architecture.md` — team/cloud profile (gateway/orchestrator, multi-tenant).
- `engine-internals.md` — internal structure of the sqlrs engine.
- `runtime-snapshotting.md` — runtime storage model, snapshot/backends (btrfs/VHDX/etc).
- `state-cache-design.md` — cache keys, triggers, retention, local store layout.
- `sql-runner-api.md` — runner API surface, timeouts, cancel, streaming.
- `liquibase-integration.md` — Liquibase provider strategy and structured logs.
- `autoscaling.md` — autoscaling controller (team/cloud) for sandboxes/cache.
- `cli_contract.md` — CLI contract and commands.
- `query-analysis-workflow-review.md` — notes on query analysis workflow.
