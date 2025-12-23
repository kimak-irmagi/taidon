# Autoscaler Requirements (Sandboxes and Snapshot Cache)

Scope: autoscaling controller for sandbox runtimes and snapshot-cache workers. Applies to Team (A2) and Cloud (B3/C4) deployments; not required for Local (A1) or Drop-in (M2) except for a tiny fixed warm pool.

## 1. Goals and Non-Goals

- Goals:
  - Keep run queue latency within SLO by scaling sandboxes horizontally.
  - Keep cache build queue short and hit ratio high by scaling builders/GC.
  - Provide warm pool to reduce cold-start for frequent flows.
  - Respect quotas/limits per org/profile; avoid resource thrash.
  - k8s-native; no custom kubelet plugins.
- Non-Goals:
  - Not doing per-statement micro-scheduling inside PostgreSQL.
  - Not implementing billing/cost optimization; only respects provided caps.
  - Not auto-tuning SQL Runner; focuses on capacity, not query plans.

## 2. Scenarios

- Team / On-Prem (A2): conservative scaling; ops may disable cluster autoscaler changes; small warm pool.
- Cloud Sharing (B3): aggressive scale-out with node group growth; larger warm pool.
- Education (C4): burst overrides near deadlines (higher warm pool/node caps).

## 3. Functional Requirements

- FR1: Consume backlog metrics (`pending_count`, `p95_age`, per-priority) from orchestrator; compute scale-out targets for sandbox pools.
- FR2: Consume active/idle sandboxes and TTLs from env-manager; adjust warm pool to low/high watermarks per profile.
- FR3: Consume cache metrics (`hit_ratio`, `build_queue_len`, `build_latency`) and scale snapshot-cache builders/GC.
- FR4: Enforce guard rails: min/max replicas per pool, max nodes per profile, per-org quotas; never exceed limits from user-profile.
- FR5: Support scale-out (increase replicas) and selective scale-up (bump VPA target or switch pool class) based on throttling/saturation signals.
- FR6: Graceful scale-in: drain pods from new assignments, wait for completion or timeout before removing capacity.
- FR7: Emit audit events (`scale_up`, `scale_down`, `warm_pool_adjusted`) with reason and metrics snapshot.
- FR8: Provide feature flags: disable warm pool, freeze scaling, cap cluster autoscaler actions.

## 4. Inputs and Signals

- Orchestrator API: `GET /queue/metrics` (pending, age, priorities); events `run_enqueued|run_started|run_finished|run_failed|run_cancelled`.
- Env-manager API: `GET /pools`, `GET /sandboxes?state=active|idle` with TTL; events `sandbox_ready|sandbox_terminated|pool_scaled`.
- Snapshot-cache API/metrics: `GET /metrics` (hit ratio, build queue len/latency); events `cache_miss_build_started|cache_miss_build_finished`.
- Telemetry (Prometheus/custom): CPU/RAM/IO for nodes/pods; throttling; warm-pool occupancy.
- User-profile: `GET /profiles`, `GET /org/{id}/limits` (quotas, priorities).

## 5. Control Actions and APIs

- Kubernetes:
  - Scale subresource: `PATCH /apis/apps/v1/.../deployments/{name}/scale`, `.../statefulsets/{name}/scale`.
  - Apply/Patch HPA/VPA objects per profile (Team/Cloud/Edu).
  - Optional node group hints (labels/taints/annotations) for fast-disk pools; cluster autoscaler only in Cloud when enabled.
- Env-manager:
  - `POST /pools/{id}/warm` (set desired warm slots).
  - `POST /pools/{id}/drain` (graceful scale-in).
  - Update pool spec (affinity/tolerations) when switching pool class.
- Snapshot-cache:
  - Scale builders/GC via HPA target metric `build_queue_len` or explicit `POST /workers/scale`.
- Audit-log:
  - Append scaling events with reason and metrics context.

## 6. Scaling Strategy (Decision Rules)

- Prefer scale-out when backlog high or queue age exceeds SLO and nodes are not saturated; grow warm pool slightly when sustained.
- Use scale-up when backlog is modest but pods are CPU-throttled or IO-saturated (heavy runs/builds); do not overuse VPA for long-lived pods—prefer pool class switch.
- If both backlog high and saturation high: scale out first; if node pressure persists, request node group growth (Cloud). Scale up only for pools flagged as needing larger memory/IO.
- Warm pool: maintain occupancy between low/high watermarks; drain excess during low load.

## 7. Policies and Profiles

- Team (A2): small warm pool; conservative HPA; no node autoscaler changes unless explicitly enabled; stricter max replicas.
- Cloud (B3): higher warm pool; allow node autoscaler; wider HPA bounds; priority-aware scaling (paid > edu > anon).
- Education (C4): temporary overrides (higher warm pool, higher max nodes) on schedule windows.

## 8. Observability and Alerts

- Metrics: queue size, p95 age, scale decisions (reason/target), warm-pool usage, cache hit ratio, build queue len/latency, throttling incidents, drain duration/count.
- Alerts: sustained backlog, thrash (oscillating scale), low hit ratio, node saturation on fast storage, failed scale operations.
- Traces/logs: correlate run_id with scale decisions when triggered by backlog.

## 9. Configuration and Safety

- Config surface: SLOs (queue age, latency), watermarks, min/max replicas, max nodes, per-profile overrides, feature flags.
- Safety: freeze mode; guard caps; dry-run option for staging; exponential backoff on failed scale actions.

## 10. Dependencies and Ordering

- Requires orchestrator, env-manager, snapshot-cache metrics and APIs to be available.
- Depends on telemetry stack (Prometheus/custom metrics API).
- Runs as control-plane deployment/cronjob with RBAC to scale resources.
