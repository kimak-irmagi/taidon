# Runtime Snapshotting Design (MVP)

This document fixes the **runtime and snapshotting architecture decisions** for the MVP of `sqlrs`.
It focuses on **how database states are materialised, snapshotted, cloned, and reused** at runtime, with a clear evolution path.

---

## 1. Goals

- Enable **frequent, cheap snapshots** (after each migration step).
- Fully control all **persisted runtime state** (DB data directories).
- Support **cache rewind** and fast branching.
- Avoid architectural assumptions about a single DB engine or version.
- Build an MVP that is simple, debuggable, and evolvable.

---

## 2. Non-Goals (MVP)

- In-database snapshotting or WAL-level manipulation.
- Restoring or continuing mid-transaction execution.
- Cross-node replication of live instances.
- Full multi-engine support beyond Postgres family.

---

## 3. Chosen Runtime Strategy

### 3.1 Path A: Externalised Persistent State

All **persistent DB state** (e.g., `PGDATA`) is:

- owned by `sqlrs`, not by containers;
- stored on the host filesystem;
- mounted into DB containers at runtime.

Containers are **stateless executors**.

### 3.2 Host Storage Strategy (by platform)

- **Linux (primary):** host-managed state store with OverlayFS copy-on-write.
- **Windows / WSL2:** snapshot backend added later; fallback is full copy.

Runtime code does not expose concrete paths: engine/adapter resolves data dirs internally and hands mounts to the runtime.
For local engine, the state store root is `<StateDir>/state-store`.

---

## 4. Container Model

### 4.1 Docker-based Runtime (Local MVP)

- Docker is used as the instance execution layer.
- Each instance runs a single DB container.
- Containers may stay running after prepare (warm instances) until run
  orchestration decides to stop them.

### 4.2 Images

Docker images contain:

- DBMS binaries (e.g., Postgres)
- entrypoint and health checks

Images **do not contain data**.

### 4.3 Supported Engines (MVP)

| Engine   | Versions | Notes                                |
| -------- | -------- | ------------------------------------ |
| Postgres | 15, 17   | Same engine family, different majors |

This validates:

- engine/version selection logic,
- absence of hardcoded assumptions.

MySQL is explicitly deferred.

---

## 5. Base States ("Zero States")

### 5.1 Definition

A **base state** is an immutable filesystem representation of an initialised DB cluster.

For Postgres:

- created via `initdb`;
- contains default databases (`postgres`, templates).

Liquibase typically operates on an existing database; explicit `CREATE DATABASE` is optional and not required for MVP.

### 5.2 Storage

Base states are stored as **CoW-capable filesystem datasets** and treated like any other immutable state.

---

## 6. Snapshotting Backend

### 6.1 Primary Backend (MVP)

- **OverlayFS layers** (Linux hosts) for copy-on-write snapshots.

Rationale:

- fast clone/snapshot
- cheap branching
- good local Linux support

### 6.2 Fallback Backend

- Any host without OverlayFS: recursive copy (MVP).
- Windows/WSL2 snapshot backend is added later.

Used when CoW FS is unavailable.

### 6.3 Pluggable Snapshotter Interface

```text
Clone(base_state) -> sandbox_state
Snapshot(sandbox_state) -> new_state
Destroy(state_or_sandbox)
Capabilities() -> { requires_db_stop, supports_writable_clone, supports_send_receive }
```

Implementation variants:

- `OverlayFSSnapshotter`
- `CopySnapshotter`
- (future) `BtrfsSnapshotter`, `ZfsSnapshotter`, `CsiSnapshotter`

### 6.4 Backend Selection Policy

- The runtime selects a snapshotter based on host capabilities and optional config override
  (e.g., `engine.config.snapshot.backend`).
- If the preferred backend is unavailable, it **falls back to** `CopySnapshotter`.
- The chosen backend is recorded in state metadata for GC/restore compatibility.

### 6.5 Snapshotter Invariants

- **Immutability:** a state is immutable after `Snapshot`.
- **Idempotent destroy:** `Destroy` is safe to call repeatedly.
- **Writable clone:** `Clone` always produces a writable sandbox independent of the base.
- **Non-destructive snapshot:** `Snapshot` must not mutate the sandbox state.

---

## 7. State Layout

```text
<StateDir>/state-store/
  engines/
    postgres/
      15/
        base/
        states/<uuid>/
      17/
        base/
        states/<uuid>/
```

- Each state is immutable once created.
- Instances use writable clones of states.
- Metadata lives in `<StateDir>/state.db`.

---

## 8. Snapshot Consistency Modes

### 8.1 DBMS-Assisted Clean Snapshot (Default)

- DBMS is paused via a connector (Postgres: `pg_ctl -m fast stop`) while the container stays up.
- Snapshot is taken by the snapshot manager.
- DBMS is restarted after snapshot.

Used for:

- CI/CD
- strict reproducibility
- Can be requested by policy or per-run flag.

### 8.2 Crash-Consistent (Optional)

- Filesystem snapshot without DB coordination.
- Relies on DB crash recovery on next start.

### 8.3 Consistency Control Boundary

Consistency mode is decided by **orchestration** (prepare/run policy), not by the snapshotter.
Snapshotters only declare whether they require a DB stop via `Capabilities().requires_db_stop`.
If a backend requires a stop, orchestration **must** pause the DBMS before calling `Snapshot`.

---

## 9. Instance Lifecycle

### 9.1 Creation

1. Select `(engine, version)`.
2. Select base state.
3. Clone base state into instance state.
4. Start container with instance state mounted.

### 9.2 Execution

- Liquibase or user commands run inside the instance.
- DB connections are proxied or exposed as needed.

### 9.3 Snapshotting

After a successful step:

1. Snapshot instance state into immutable state.
2. Register state in cache index.

### 9.4 Teardown and Cooldown

- After prepare, the container stays running and the instance is recorded as warm.
  Run orchestration will decide when to stop warm instances.
- If a warm instance container is missing, run orchestration recreates the
  container from the preserved runtime data directory; missing runtime data is
  treated as an error (no state rebuild).

---

## 10. Multi-Version Safety

The following rules apply:

- Engine and version are **explicit parameters** everywhere.
- No default engine/version assumptions in runtime code.
- All paths and images are resolved via engine adapters.

This prevents accidental coupling to a single engine or layout.

### 10.1 Engine Process Model

- CLI is thin; it auto-spawns/communicates with the engine process (local daemon) that owns state store, snapshotting, and container lifecycle.
- Engine adapter hides filesystem layout (`PGDATA` roots, state store paths) and provides mount specs to the runtime; callers never touch raw paths.

---

## 11. Evolution Path

### Phase 1 (MVP)

- Docker + OverlayFS
- Postgres 15 & 17
- Local state store

### Phase 2

- Remote/shared cache
- ZFS / send-receive
- Pre-warmed pinned states
- Windows/WSL snapshot backend

### Phase 3

- Kubernetes (CSI snapshots)
- MySQL engine adapter
- Cloud-scale eviction policies

---

## 12. Open Questions

- Minimum viable abstraction for volume handles across snapshotters?
- How aggressively to snapshot large seed steps?
- Default cooldown policy for instances?
