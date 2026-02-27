# sqlrs cache capacity control (draft)

This guide defines operator-facing cache capacity controls for local engine
state snapshots.

Status: draft design (not implemented yet).

## 1. Purpose

Prevent unbounded growth of the local state cache by enforcing size limits and
automatic eviction.

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

## 4. Error Codes (planned)

- `cache_enforcement_unavailable`
  - usage cannot be measured while strict capacity enforcement is enabled.
- `cache_full_unreclaimable`
  - reclaimable states are insufficient to satisfy watermark and/or reserve
    constraints.
- `cache_limit_too_small`
  - effective cache limit is too small to materialize even one prepare state.
