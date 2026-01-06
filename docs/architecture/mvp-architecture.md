# Taidon Architecture (Education MVP)

Goal: launch a minimal viable platform for SQL learning scenarios with isolated sandboxes, deployed on-prem in Kubernetes (at minimum, minikube for local debugging).
Interaction and component diagrams: [`docs/architecture/diagrams.md`](docs/architecture/diagrams.md).

## 1. Users and isolation

- Two user types: anonymous (~900 concurrent), registered (~100). Each gets a separate sandbox (DB container + snapshots).
- Two-level model at start: sandbox -> account. Later: orgs -> accounts -> sandboxes (GitHub-like hierarchy).
- Secrets (Git tokens) are stored in k8s Secrets; access is limited to sandbox/account scope.

## 2. MVP service set

- `frontend/main`: SPA for editor/results.
- `backend/gateway`: BFF for the frontend; auth termination, rate limiting, routing to services.
- `idp`: authentication (email/password/OIDC), token session management for anonymous/registered users.
- `user-profile`: accounts, sandboxes, quotas/limits (no billing).
- `vcs-sync`: work with projects on local FS and optional Git sync (pull/push on demand), secrets storage.
- `env-manager`: sandbox orchestration: create/delete k8s Pod/StatefulSet with PostgreSQL, mount snapshots, enforce resource limits and network policies.
- `snapshot-cache`: snapshot management (by SQL chain hash), retention/eviction policy, CoW storage integration.
- `sql-runner`: deterministic replay of SQL chain: compute head/tail, request snapshot from `snapshot-cache`, run head, create new layer.
- `audit-log`: user/system action logging.
- `telemetry`: metrics/traces (Prometheus scrape), export to Grafana.
- Deferred: `billing`, `scheduler` (background maintenance can be done via manual jobs or k8s cronjob).

## 3. Key flows

- **SQL session run**: frontend -> gateway -> sql-runner. `sql-runner` computes tail hash, requests snapshot from `snapshot-cache`. If found, `env-manager` boots sandbox from snapshot and head is executed. If not, tail is replayed, the layer is stored in cache, then head is executed. Results/errors return to frontend.
- **Sandbox management**: create/delete/restart by user request or TTL; `env-manager` applies ResourceQuota/LimitRange/NetworkPolicy and mounts snapshot volume.
- **Project workflow**: local directory as source of truth. On request, Git sync via `vcs-sync` (pull/push), using k8s Secret for tokens. Storage model: SQL script tree `init/seed/test/bench`.

## 4. Data and storage

- **Platform metadata**: separate PostgreSQL (control plane) for users, sandboxes, snapshot descriptions, audit.
- **Snapshots**: CoW layers for PostgreSQL containers. On-prem: local PV (qcow2/ZFS/btrfs/LVM-thin) with periodic materialization; minikube: hostPath/local volume. Future: S3-compatible object storage for layers.
- **Logs/audit**: centralized logs (stdout -> Loki/ELK), audit in control plane table.
- **Secrets**: k8s Secrets + optional KMS/Vault integration later.

## 5. Deployment / infrastructure

- **Clusters**: on-prem k8s for prod/stage; minikube for dev. Same manifests/Helm charts, different values.
- **Network boundaries**: gateway exposed externally; services are ClusterIP. NetworkPolicy restricts access to Postgres sandboxes to `sql-runner` only.
- **Resources**: per-sandbox CPU/RAM limits; node affinity/taints for nodes with fast disks for snapshots.
- **CI/CD**: build containers, deploy via Helm/ArgoCD (when available).

## 6. Observability

- Prometheus scrape: gateway, sql-runner, env-manager, snapshot-cache, idp, user-profile.
- Core metrics: error rate (5xx/4xx), cache hit ratio (tail hash), sandbox startup latency and head execution latency, active sandbox count, CPU/RAM/IO usage on snapshot nodes, cache size/eviction rate.
- Alerts: low hit ratio, increasing sandbox startup time, PV saturation for snapshots, spikes in auth/Git errors.
- Grafana dashboards for requests/sandboxes/cache.

## 7. MVP constraints and simplifications

- PostgreSQL only, no extensions; single engine version.
- No billing; simple quotas (sandbox count, resource limits).
- No background scheduler; cache maintenance via `snapshot-cache` calls on events or periodic k8s cronjob.
- External integrations limited to Git (optional).
- Orgs and complex RBAC later (account/sandbox for now).

## 8. Open questions / risks

- Snapshot layer format for on-prem: btrfs/ZFS/LVM-thin/overlay2 - choose based on cluster availability and mount time.
- Cache GC strategy without full scheduler: is k8s cronjob enough for MVP?
- TTL policy for anonymous sandboxes (resource control).
- Required cold start time and target SLA for response - to tune cache/limits.
