# State Cache Capacity Control (Local MVP Hardening)

This document defines the bounded-cache design for local engine state snapshots.
It complements:

- [`state-cache-design.md`](state-cache-design.md)
- [`runtime-snapshotting.md`](runtime-snapshotting.md)
- [`statefs-component-structure.md`](statefs-component-structure.md)
- [`engine-internals.md`](engine-internals.md)

## 1. Problem

The local state cache currently keeps snapshots indefinitely unless they are
manually removed. This creates unbounded growth of `<state-store-root>`, which
is operationally unsafe for long-lived workspaces and CI agents.

## 2. Product Behavior (User-facing Contract)

The cache must behave predictably:

1. Operator configures a single logical cache budget.
2. Effective cache max is coupled to the capacity of the state-store filesystem.
3. When usage crosses the high watermark, eviction starts automatically.
4. Eviction also starts when free space drops below a reserve floor.
5. Eviction continues until usage drops below the low watermark and free space
   is back above reserve.
6. States that are actively needed are never evicted by normal policy.
7. If the engine cannot reclaim enough space, prepare fails with a clear,
   actionable error.

This gives a strict capacity guarantee instead of best-effort cleanup.

## 3. Configuration Contract

Capacity controls are exposed via existing server config APIs (`sqlrs config`).

Planned paths:

- `cache.capacity.maxBytes`: integer, nullable
  - `null` or `0` means "use store-coupled max only".
- `cache.capacity.reserveBytes`: integer, nullable
  - if `null`, default reserve is `max(10 GiB, 10% of store total bytes)`.
- `cache.capacity.highWatermark`: number in `(0,1]`, default `0.90`
- `cache.capacity.lowWatermark`: number in `(0, high)`, default `0.80`
- `cache.capacity.minStateAge`: duration string, default `10m`

Effective limit computation:

1. `store_total_bytes` is measured from the filesystem hosting the state store.
2. `effective_max_from_store = max(0, store_total_bytes - reserveBytes)`.
3. If `cache.capacity.maxBytes > 0`, `effective_max = min(maxBytes, effective_max_from_store)`.
4. Else, `effective_max = effective_max_from_store`.

Notes:

- For `--store image|device` (btrfs VHDX/device), `store_total_bytes` is the
  mounted virtual disk capacity.
- For `--store dir` (copy/overlay/root btrfs), `store_total_bytes` is the host
  filesystem capacity at the store mount.

Validation rules:

- `maxBytes >= 0`
- `reserveBytes >= 0`
- `0 < lowWatermark < highWatermark <= 1`
- `minStateAge >= 0`

## 4. Eviction Eligibility Rules

A state is eligible only if all conditions hold:

1. `refcount == 0` (no active instance uses it)
2. no children (`leaf` in state DAG)
3. state age >= `minStateAge`
4. not explicitly protected by retention metadata (future pin/class policy)

This keeps correctness simple and backend-independent.

## 5. Selection Policy (v1)

Policy v1 favors explainability over complexity:

1. Build candidate set from eligibility rules.
2. Rank by:
   - older `last_used_at` first,
   - then larger `size_bytes` first.
3. Delete in order until usage <= low watermark.

Future versions may add replay-cost/priority scoring, but v1 must remain easy to
reason about in logs.

## 6. Triggers and Execution Model

Eviction runs on these triggers:

1. after successful state creation (`Snapshot` + `CreateState` commit),
2. on engine startup recovery pass,
3. before `state_execute` and before `Snapshot` when strict capacity mode is on,
4. optional periodic background check.

Execution must be serialized by a global evictor lock
(`.evict.lock` under state-store root) to avoid concurrent eviction races.

## 7. Backend Responsibilities Across Snapshot Mechanisms

The policy is uniform across `copy`, `overlayfs`, and `btrfs`; backend-specific
logic is limited to storage accounting and deletion primitives.

### 7.1 Storage usage measurement

- `copy` / `overlayfs`: recursive filesystem size walk.
- `btrfs`: prefer `btrfs filesystem du`-based estimation; fallback to recursive walk.
- future `zfs`: use dataset-native used-space metrics.

The evictor additionally measures:

- `store_total_bytes`
- `store_free_bytes`

The evictor must always log both:

- estimated bytes freed by candidate metadata,
- observed before/after store usage.

### 7.2 Deletion primitive

Physical deletion is executed through existing deletion semantics (statefs-aware
path removal), preserving backend-specific safety behavior:

- btrfs subvolume deletion before directory removal,
- overlay/copy path removal fallback logic.

## 8. Metadata and Storage Schema Changes

State metadata must be extended to support deterministic policy decisions:

- `last_used_at` (timestamp)
- `use_count` (integer)
- `min_retention_until` (timestamp, nullable)
- `evicted_at` (timestamp, nullable, optional if keeping tombstones)
- `eviction_reason` (text, nullable, diagnostic)

`size_bytes` already exists and is retained as the primary per-state size signal.

To handle "single-state too large" corner cases, the engine also keeps rolling
observations for required build space per `(image_id, prepare_kind)`:

- latest successful peak build bytes,
- minimum observed successful build bytes.

## 9. Component Integration

### 9.1 New internal component

Add `prepare.cacheEvictor` (or sibling package) that:

- reads capacity config,
- checks store usage,
- computes candidate list,
- performs deletion and emits events/diagnostics.

### 9.2 Existing components

- `prepare.snapshotOrchestrator` triggers eviction after successful snapshot
  materialization.
- `deletion.Manager` remains the authority for safe state removal semantics.
- `httpapi` and CLI diagnostics surface eviction outcomes.

## 10. Failure Semantics

When capacity control is enabled, enforcement is strict:

1. If usage cannot be measured reliably, prepare fails with
   `cache_enforcement_unavailable`.
2. If eviction cannot satisfy constraints because remaining states are
   protected/in-use/non-leaf, prepare fails with `cache_full_unreclaimable`.
3. If effective limit is too small to materialize even one state, prepare fails
   with `cache_limit_too_small`.

`cache_full_unreclaimable` must include a machine-readable reason, including:

- `usage_above_high_watermark`
- `physical_free_below_reserve`

`cache_limit_too_small` must include:

- `effective_max_bytes`
- `observed_required_bytes` (if known)
- `recommended_min_bytes` (computed lower bound)

If an unexpected `ENOSPC` still occurs inside execution/snapshot despite
preflight, it must be normalized to one of the above errors and include the
failure phase (`prepare_step`, `snapshot`, `metadata_commit`).

Both failures must include machine-readable details (blocked reasons, bytes
needed, bytes reclaimable).

## 11. Observability

Emit structured events and logs:

- `cache_check` (usage, thresholds, trigger)
- `cache_evict_candidate` (state_id, reason, size_bytes, rank signals)
- `cache_evict_result` (state_id, success/failure, bytes_before, bytes_after)
- `cache_evict_summary` (evicted_count, freed_bytes, blocked_count)

Minimum operator-visible status should include:

- current usage bytes,
- configured max/high/low,
- last eviction run summary.

## 12. Rollout Plan

1. Implement config and schema extensions.
2. Implement eligibility + HWM/LWM eviction core.
3. Wire trigger after snapshot commit.
4. Add startup check and diagnostics.
5. Add backend-specific usage estimators and cross-platform tests.

## 13. Non-Goals for v1

- Global optimal eviction over full DAG.
- Cross-workspace/shared cache balancing.
- Aggressive pin override under normal operation.
