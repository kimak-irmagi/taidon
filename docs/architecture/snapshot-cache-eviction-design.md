# Snapshot Cache Eviction in Versioned Database Storage

## 1. Context and Problem Statement

In Taidon, a database state is treated as a **versioned artifact**, similar to commits in a version control system. Each database version is implemented as a snapshot on top of a Copy-on-Write (CoW) filesystem (e.g. ZFS).

Snapshots form a **directed acyclic graph (DAG)**:
- nodes represent database snapshots,
- edges represent parent → child relationships created by snapshot/clone operations.

Typical CI/CD workloads generate snapshots at high frequency. These snapshots:
- are created and destroyed in large numbers,
- differ significantly in reuse frequency and value,
- compete for limited storage capacity.

This document describes a practical approach to **automatic cache size management** for such snapshots.

---

## 2. Constraints and Requirements

### 2.1 Architectural Constraints

1. **CoW Filesystem Backend**
   - Storage relies on block sharing provided by a CoW filesystem.
   - Space efficiency is achieved by sharing unchanged blocks between snapshots.

2. **Correctness Guarantees**
   - Snapshot removal must not break access to remaining snapshots.
   - Only **leaf snapshots** in the DAG are eligible for eviction.

3. **Pinning Mechanism**
   - Snapshots may be excluded from eviction via pinning:
     - manual pin (user or policy),
     - active usage (mounted or running environment),
     - time-based protection (grace period / TTL).

4. **Persistent Metadata**
   - Logical snapshot metadata may be retained even after physical deletion,
     to support accounting, access statistics, and restoration.

---

### 2.2 Operational Requirements

1. **Stability**
   - The system must avoid thrashing (rapid creation followed by immediate deletion).

2. **Predictability**
   - Freed storage should correlate with eviction decisions in an explainable way.

3. **Local Decision-Making**
   - Eviction decisions must rely on locally available metrics,
     without global recomputation of the entire snapshot graph.

4. **Scalability**
   - The approach must remain practical with thousands or tens of thousands of snapshots.

---

## 3. Cache Management Model

A **high watermark / low watermark (HWM/LWM)** model is used:

- When used storage exceeds the **high watermark**, eviction is triggered.
- Eviction continues until storage usage drops below the **low watermark**.
- Eviction consists of repeated snapshot removal operations.

---

## 4. Eviction as a Primitive Operation

### 4.1 Definition

An **eviction operation** consists of:
1. selecting an eviction candidate snapshot,
2. physically deleting it from the CoW filesystem,
3. recording the outcome (freed space, time, side effects).

---

### 4.2 Candidate Set

A snapshot may be considered for eviction if all of the following hold:
- it is a **leaf** in the snapshot DAG,
- it is **not pinned**,
- it is **not actively used**,
- it is older than a configured minimum age.

---

## 5. Target Metrics

### 5.1 High-Level Objective

The eviction policy aims to **maximize reclaimed storage while minimizing expected future cost**.

Intuitively:

> Remove snapshots that free a lot of space and are unlikely to be needed again,
> or are cheap to restore if they are needed.

---

### 5.2 Measurable Proxy Metrics

#### 1. Storage Reclamation Estimate

`freed_space(snapshot)`

Estimated using CoW filesystem statistics:
- for ZFS, the `used` value of a leaf snapshot;
- analogous metrics for other CoW filesystems.

For leaf snapshots, this value is a good approximation of their true storage contribution.

---

#### 2. Access Probability

`access_frequency(snapshot)`

Derived from:
- direct accesses to the snapshot,
- optionally, accesses to its descendants,
- with time decay applied.

This metric estimates the probability that the snapshot will be needed again.

---

#### 3. Restoration Cost

An estimate of the cost incurred if the snapshot must be recreated after eviction.

Initially modeled using:
- historical creation cost,
- access frequency,
- coarse heuristics of rebuild complexity.

For leaf snapshots, restoration cost is treated as **local**, independent of other snapshots.

---

### 5.3 Composite Scoring

Candidates are ranked using a composite heuristic that balances:
- expected storage benefit,
- probability of reuse,
- expected restoration cost.

The exact scoring function is subject to tuning and experimentation.

---

## 6. Eviction Algorithm Variants

### 6.1 Greedy Eviction

1. Collect all eligible leaf snapshots.
2. Compute a score for each candidate.
3. Evict snapshots with the best score until LWM is reached.

Pros:
- simple and transparent.

Cons:
- sensitive to metric noise.

---

### 6.2 Batch-Based Eviction

- Eviction is performed in batches.
- Scores are recomputed only between batches.

Pros:
- lower overhead,
- more stable behavior.

---

### 6.3 Age-Tiered Eviction

- Snapshots are grouped by age.
- Eviction starts from the oldest tier.
- Scoring is applied within each tier.

Pros:
- protects recent snapshots from premature eviction.

---

### 6.4 Class-Based Retention

- Snapshots are assigned to retention classes:
  - ephemeral (CI runs),
  - semi-persistent (PR-related),
  - persistent (releases).
- Eviction is restricted within classes.

Pros:
- easier to reason about product behavior,
- reduces risk of removing critical snapshots.

---

## 7. Known Limitations

- The approach is heuristic-based and not globally optimal.
- Storage ownership is approximated rather than tracked exactly.
- Only leaf snapshots are considered for eviction.
- Excessive pinning may prevent sufficient space reclamation.

These limitations are considered acceptable for an initial production implementation.

