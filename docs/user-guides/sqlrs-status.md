# sqlrs status

## Overview

`sqlrs status` checks engine health and reports local runtime diagnostics.

The bounded-cache hardening slice extends `status` in two layers:

- the default `status` output includes a compact cache summary with the key
  occupancy indicators;
- `status --cache` expands that summary into full bounded-cache diagnostics.

Current status:

- base health/status output is implemented;
- compact cache summary and `--cache` expansion are implemented in the current
  local CLI.

---

## Command Syntax

```text
sqlrs status [--cache]
```

Rules:

- `status` does not accept positional arguments.
- default `status` includes a compact cache summary when cache diagnostics are
  available.
- `--cache` requests the full cache diagnostics block.
- global `--output human|json` continues to control the output format.

---

## Default Output

Without `--cache`, `sqlrs status` reports:

- engine health (`ok` / unavailable)
- endpoint, profile, mode
- client / engine version details
- local dependency readiness in local mode
- a compact cache summary

The compact cache summary is intended to answer the quick operator question:
"how full is the cache right now?"

### Human Output

Human output keeps the existing `key: value` layout and appends compact cache
summary lines such as:

- `cache.usageBytes`
- `cache.effectiveMaxBytes`
- `cache.storeFreeBytes`
- `cache.stateCount`
- `cache.pressureReasons` (only when non-empty)

If compact cache summary collection fails, `status` should still report health
and print cache summary as unavailable or emit a warning.

### JSON Output

With `--output json`, the default result keeps the existing status object and
adds a compact `cacheSummary` object when available:

```json
{
  "ok": true,
  "endpoint": "http://127.0.0.1:5107",
  "mode": "local",
  "version": "dev",
  "cacheSummary": {
    "usageBytes": 2147483648,
    "effectiveMaxBytes": 10737418240,
    "storeFreeBytes": 12884901888,
    "stateCount": 14,
    "pressureReasons": []
  }
}
```

If compact cache summary is unavailable, `cacheSummary` may be omitted or set to
`null`, while the main status result still reflects engine health.

---

## Full Cache Diagnostics Mode

Use `--cache` when you need the full bounded-cache diagnostics payload:

```bash
sqlrs status --cache
```

This mode expands the compact summary with thresholds, reclaimability, and the
latest eviction result.

### Human Output

Human output keeps the compact cache summary and additionally appends detailed
cache-prefixed lines such as:

- `cache.reserveBytes`
- `cache.highWatermark`
- `cache.lowWatermark`
- `cache.minStateAge`
- `cache.storeTotalBytes`
- `cache.reclaimableBytes`
- `cache.blockedCount`
- `cache.lastEviction.completedAt`
- `cache.lastEviction.trigger`
- `cache.lastEviction.evictedCount`
- `cache.lastEviction.freedBytes`

When there has been no completed eviction yet, `cache.lastEviction.*` fields are
omitted in human output.

### JSON Output

With `--output json`, `status --cache` keeps `cacheSummary` and additionally
adds a `cacheDetails` object:

```json
{
  "ok": true,
  "endpoint": "http://127.0.0.1:5107",
  "mode": "local",
  "version": "dev",
  "cacheSummary": {
    "usageBytes": 2147483648,
    "effectiveMaxBytes": 10737418240,
    "storeFreeBytes": 12884901888,
    "stateCount": 14,
    "pressureReasons": []
  },
  "cacheDetails": {
    "reserveBytes": 1073741824,
    "highWatermark": 0.9,
    "lowWatermark": 0.8,
    "minStateAge": "10m",
    "storeTotalBytes": 21474836480,
    "reclaimableBytes": 536870912,
    "blockedCount": 3,
    "lastEviction": {
      "completedAt": "2026-03-09T12:00:00Z",
      "trigger": "post_snapshot",
      "evictedCount": 2,
      "freedBytes": 268435456,
      "blockedCount": 3,
      "reclaimableBytes": 536870912,
      "usageBytesBefore": 2415919104,
      "usageBytesAfter": 2147483648,
      "freeBytesBefore": 12616466432,
      "freeBytesAfter": 12884901888
    }
  }
}
```

---

## Validation and Errors

- `status` still fails if the engine is unavailable or unhealthy.
- default compact cache summary is best-effort and may degrade to unavailable.
- `status --cache` is strict: it fails if full cache diagnostics cannot be
  collected reliably.

Full cache diagnostics must not silently fall back to guessed values.

---

## Related Commands

- `sqlrs ls --states` shows cached states as objects.
- `sqlrs ls --states --cache-details` is the per-state companion to
  `sqlrs status --cache`.
- `sqlrs config get cache.capacity.*` shows configured policy inputs.

---

## Examples

Basic health check with compact cache summary:

```bash
sqlrs status
```

Health plus full bounded-cache details:

```bash
sqlrs status --cache
```

Machine-readable full cache diagnostics:

```bash
sqlrs --output json status --cache
```
