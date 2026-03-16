# sqlrs cache capacity control

This guide defines operator-facing cache capacity controls for local engine
state snapshots.

Status: core enforcement, operator-facing diagnostics, persisted `size_bytes`
metadata, and release cache-pressure coverage are implemented for the current
local profile. Broader matrix expansion remains optional future hardening.

## 1. Purpose

Prevent unbounded growth of the local state cache by enforcing size limits and
automatic eviction.

Current implementation covers:

- `cache.capacity.*` configuration via `sqlrs config`
- strict enforcement around prepare/snapshot phases
- deterministic eviction of unreferenced leaf states older than `minStateAge`
- structured errors when the cache cannot be reclaimed enough
- `sqlrs status` compact cache summary and `sqlrs status --cache` detailed diagnostics
- `sqlrs ls --states --cache-details` per-state cache metadata
- persisted `size_bytes` metadata for newly materialized states
- dedicated release cache-pressure scenario coverage for the local profile

Remaining work:

- expand cache-pressure release coverage to more platform/backend combinations
  if that becomes operationally important

## 1.1 Operator Diagnostics

The bounded-cache hardening slice introduces three operator-facing views:

```text
sqlrs status
sqlrs status --cache
sqlrs ls --states --cache-details
```

Current semantics:

- `sqlrs status`
  - shows a compact cache summary together with the normal health report;
- `sqlrs status --cache`
  - expands the compact summary into full occupancy, threshold, reclaimability,
    and last-eviction details;
- `sqlrs ls --states --cache-details`
  - shows per-state cache metadata such as `last_used_at`, `use_count`, and
    `min_retention_until`;
- `sqlrs config get cache.capacity.*`
  - remains the source of configured policy values.

These diagnostics are designed to complement, not replace, the existing config
and state-listing commands.

## 2. Configuration Paths

Use existing `sqlrs config` commands.

```text
sqlrs config set cache.capacity.maxBytes <integer|null>
sqlrs config set cache.capacity.reserveBytes <integer|null>
sqlrs config set cache.capacity.highWatermark <number>
sqlrs config set cache.capacity.lowWatermark <number>
sqlrs config set cache.capacity.minStateAge "<duration>"
```

Examples:

```text
sqlrs config set cache.capacity.maxBytes 32212254720
sqlrs config set cache.capacity.reserveBytes 10737418240
sqlrs config set cache.capacity.highWatermark 0.90
sqlrs config set cache.capacity.lowWatermark 0.80
sqlrs config set cache.capacity.minStateAge "\"10m\""
```

Use store-coupled mode:

```text
sqlrs config set cache.capacity.maxBytes 0
```

## 3. Expected Behavior

1. If usage crosses `highWatermark * effectiveMax`, eviction starts.
2. If free space drops below `reserveBytes`, eviction starts even when usage is
   below the watermark threshold.
3. Eviction stops when usage drops below `lowWatermark * effectiveMax` and free
   space rises above reserve.
4. Eligible states are unreferenced leaf states older than `minStateAge`.
5. If enough space cannot be reclaimed, prepare fails with a structured error.

`usage` is measured from cached state trees under
`<state_store_root>/engines/*/*/states`. Transient runtime job directories under
`<state_store_root>/jobs/*/runtime` are excluded from this usage signal.

`effectiveMax` is coupled to store capacity:

```text
effectiveMaxFromStore = store_total_bytes - reserveBytes
effectiveMax = min(maxBytes, effectiveMaxFromStore)   (when maxBytes > 0)
effectiveMax = effectiveMaxFromStore                  (when maxBytes is null/0)
```

## 4. Error Codes

- `cache_enforcement_unavailable`
  - usage cannot be measured while strict capacity enforcement is enabled.
- `cache_full_unreclaimable`
  - reclaimable states are insufficient to satisfy watermark and/or reserve
    constraints.
- `cache_limit_too_small`
  - effective cache limit is too small to materialize even one prepare state.

## 5. Diagnostics Fields

### 5.1 `sqlrs status`

Compact cache summary fields:

- `usageBytes`
- `effectiveMaxBytes`
- `storeFreeBytes`
- `stateCount`
- `pressureReasons`

### 5.2 `sqlrs status --cache`

Additional detailed fields:

- `reserveBytes`
- `highWatermark`
- `lowWatermark`
- `minStateAge`
- `storeTotalBytes`
- `reclaimableBytes`
- `blockedCount`
- `lastEviction`

`lastEviction` is returned as an object containing:

- `completedAt`
- `trigger`
- `evictedCount`
- `freedBytes`
- `blockedCount`
- `reclaimableBytes`
- `usageBytesBefore`
- `usageBytesAfter`
- `freeBytesBefore`
- `freeBytesAfter`

### 5.3 `sqlrs ls --states --cache-details`

Per-state cache metadata exposed by `sqlrs ls --states --cache-details`:

- `size_bytes`
  - persisted snapshot size in bytes for states created after size tracking is enabled;
  - older states may omit this field until they are rebuilt or explicitly remeasured;
- `last_used_at`
- `use_count`
- `min_retention_until`

Eviction uses `size_bytes` as the primary ranking signal. If a state has no stored size yet, the engine may fall back to a live filesystem measurement during eviction so strict capacity enforcement still works for legacy cache entries.
