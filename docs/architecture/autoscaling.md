# Autoscaling for Taidon Sandboxes and Cache

Scope: controller for elastic capacity of sandbox runtimes and snapshot-cache workers. Applies to Team (A2) and Cloud (B3/C4) deployments; not required for Local MVP.

## 1. Deployment Model

- **k8s-native control loop** (preferred): controller/cronjob in control plane namespace that adjusts:
  - HPA/VPA for `env-manager` managed sandbox pods/pools.
  - HPA for snapshot-cache builder/GC workers.
  - Cluster autoscaler hints (node group min/max) via labels/taints for fast-disk nodes.
- **Warm pool**: maintain a small number of pre-bound sandboxes (empty DB + base snapshot) to reduce cold start.
- No custom kubelet plugins; uses Kubernetes API and metrics APIs only.

## 2. Inputs and Signals

- Queue length / backlog from `orchestrator` (pending runs).
- Active sandboxes / occupancy from `env-manager`.
- Cache metrics from `snapshot-cache`: hit ratio, build latency.
- Resource metrics from `telemetry`: CPU/RAM/IO on nodes hosting CoW volumes.
- Policy/profile from `user-profile`: org quotas, priority tiers (paid/edu/anon).

## 3. Control Actions

- Scale sandbox pools up/down (HPA) based on backlog and latency SLOs.
- Scale cache builders/GC workers based on miss rate and build queue.
- Adjust warm-pool size per profile (Team/Cloud/Edu).
- Respect guard rails: min/max nodes, per-tenant quotas, affinity to fast storage.
- Graceful drain: stop scheduling new runs on pods slated for scale-in; wait for completion or timeout.

## 4. Interfaces and Events

Reads:

- Prometheus/custom metrics: CPU/RAM/IO per node (fast-disk pool), pod throttling, warm-pool occupancy.
- Orchestrator: `GET /queue/metrics` (pending count, p95 age, per-priority buckets); events `run_enqueued|run_started|run_finished|run_failed|run_cancelled`.
- Env-manager: `GET /pools`, `GET /sandboxes?state=active|idle`, TTLs; events `sandbox_ready|sandbox_terminated|pool_scaled`.
- Snapshot-cache: `GET /metrics` (hit ratio, build queue length/latency); events `cache_miss_build_started|cache_miss_build_finished`.
- User-profile: `GET /profiles`, `GET /org/{id}/limits` for quotas/priorities.

Writes:

- Kubernetes scale subresources: `PATCH /apis/apps/v1/.../deployments/{name}/scale`, `.../statefulsets/{name}/scale` (env-manager pools, cache builders/GC).
- HPA/VPA specs apply/patch per profile (Team/Cloud/Edu).
- Env-manager: `POST /pools/{id}/warm` (set warm slots), `POST /pools/{id}/drain` (graceful scale-in), pool spec updates (affinity to fast storage).
- Snapshot-cache: trigger worker scale via HPA target metrics (build_queue_len) or explicit `POST /workers/scale`.
- Audit-log: emit `scale_up|scale_down|warm_pool_adjusted` with reason/backlog/hit_ratio.

## 5. Policies and Defaults

- Team (A2): conservative scaling, small warm pool, no cluster-autoscaler changes by default (ops-managed).
- Cloud (B3): aggressive scale-out with caps; dynamic node group growth; higher warm pool.
- Education bursts: temporary overrides to warm pool and node caps around deadlines.

## 6. Scaling Strategies (Width vs Depth)

- **Scale out (width, preferred first)**:
  - When backlog (pending runs) > threshold or p95 queue age > SLO.
  - When cache miss/build queue grows but nodes are not saturated (IO/CPU ok).
  - Action: increase replicas (env-manager pools, cache builders); optionally grow warm pool; if node pressure high, request node group up via cluster autoscaler (Cloud only).
- **Scale up (depth, selective)**:
  - When pods are CPU-throttled or IO-saturated and backlog is low/moderate (few heavy runs).
  - When cache builders hit memory/IO limits on single large build.
  - Action: raise VPA target or switch pool class to a bigger flavor (Team/Cloud profiles). Avoid aggressive VPA for long-running pods; prefer pool flavor switch + redeploy.
- **Selection rules**:
  - If backlog high AND nodes under capacity -> scale out.
  - If backlog modest AND pod throttling high -> scale up (or move to larger pool class), keep replica count.
  - If both high (backlog + saturation) -> scale out first, then consider node group up; scale up only for pools known to need larger memory/IO.
  - Warm pool tuning is orthogonal: keep occupancy between low/high watermarks; drain excess when quiet.
- **Guard rails**: per-profile min/max replicas, max node count, per-tenant quotas; never exceed org limits from `user-profile`.

## 7. Observability and Safety

- Metrics: queue size, scale decisions, warm-pool usage, eviction/drain counts, cache hit ratio under load.
- Alerts: sustained backlog, thrash (scale up/down oscillation), low hit ratio, node saturation on fast disks.
- Feature flags: disable warm pool; cap cluster autoscaler; freeze scaling during incidents.
