# ADR: state cache capacity and eviction policy (local engine)

Status: Accepted
Date: 2026-02-22

## Decision Record 1: enforce bounded local cache with HWM/LWM and leaf-only eviction

- Timestamp: 2026-02-22T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should Taidon enforce cache size limits while preserving safe
  snapshot semantics across copy/overlayfs/btrfs backends?
- Alternatives:
  - Keep unbounded cache and rely on manual `rm` cleanup.
  - Enforce hard cap by deleting arbitrary states when above limit.
  - Use HWM/LWM policy with eligibility constraints and deterministic eviction
    order.
- Decision: Adopt bounded cache with explicit capacity config, high/low
  watermarks, and strict enforcement:
  - trigger eviction when usage crosses high watermark;
  - reclaim until usage is below low watermark;
  - evict only unreferenced leaf states older than a minimum age;
  - fail prepare with structured errors when space cannot be reclaimed.
- Rationale: This provides predictable operator-facing behavior, prevents
  unbounded growth, and keeps correctness independent of backend-specific
  snapshot mechanics.

## Decision Record 2: integrate eviction through existing deletion/statefs safety paths

- Timestamp: 2026-02-22T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Where should physical deletion be executed to guarantee consistent
  behavior across snapshot backends?
- Alternatives:
  - Add backend-specific ad-hoc deletion logic directly in eviction code.
  - Reuse existing deletion manager and statefs removal semantics.
- Decision: Route state removal through existing deletion/statefs-safe paths and
  keep backend differences limited to usage estimation.
- Rationale: This avoids duplicated destructive logic, preserves current
  btrfs-specific subvolume handling, and reduces regression risk.

## Decision Record 3: couple effective cache max to state-store capacity

- Timestamp: 2026-02-22T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should cache limit be independent from virtual disk size (`--store
  image|device`) and filesystem capacity?
- Alternatives:
  - Keep separate independent limits for cache and store size.
  - Tie cache max strictly to store/filesystem capacity with optional upper cap.
- Decision: Define a store-coupled effective cache max:
  - measure `store_total_bytes` for the filesystem hosting state-store;
  - reserve a free-space floor (`reserveBytes`);
  - compute `effective_max_from_store = store_total_bytes - reserveBytes`;
  - if `maxBytes` is set, use `min(maxBytes, effective_max_from_store)`,
    otherwise use `effective_max_from_store`.
- Rationale: Keeps a single logical capacity model for operators while
  preserving safety headroom for runtime/temp growth.

## Decision Record 4: enforce reserve floor and normalize ENOSPC into stable errors

- Timestamp: 2026-02-22T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What should happen when physical free space is lower than the
  effective cache max, and when limits are too small for even one prepare state?
- Alternatives:
  - Let operations fail where ENOSPC happens and return raw runtime errors.
  - Add preflight checks + eviction-on-pressure + normalized error contract.
- Decision:
  - trigger eviction on either watermark pressure or physical free-space reserve
    violation;
  - preflight before state execution and snapshot stages;
  - map unexpected ENOSPC to stable structured errors with phase details;
  - return `cache_limit_too_small` when effective limits cannot fit a single
    state build.
- Rationale: Prevents opaque runtime failures and gives deterministic,
  operator-actionable diagnostics for both logical and physical pressure paths.
