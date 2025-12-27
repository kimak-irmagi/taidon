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
- Cross-node replication of live sandboxes.
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

- **Linux (primary):** host-managed store on btrfs subvolumes (preferred).
- **Windows / WSL2:** host-managed VHDX; use btrfs inside if available, otherwise `copy/link-dest` fallback.

Runtime code does not expose concrete paths: engine/adapter resolves data dirs internally and hands mounts to the runtime.

---

## 4. Container Model

### 4.1 Docker-based Runtime (Local MVP)

- Docker is used as the sandbox execution layer.
- Each sandbox runs a single DB container.
- Containers are short-lived and disposable.

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

- **btrfs subvolume snapshots** (Linux hosts; WSL2 if btrfs is available inside VHDX)

Rationale:

- fast clone/snapshot
- cheap branching
- good local Linux support

### 6.2 Fallback Backend

- Windows/WSL2 without btrfs: VHDX + `copy/link-dest` snapshotting
- Any host without CoW FS: recursive copy / rsync-based snapshotting

Used when CoW FS is unavailable.

### 6.3 Pluggable Snapshotter Interface

```text
Clone(base_state) -> sandbox_state
Snapshot(sandbox_state) -> new_state
Destroy(state_or_sandbox)
```

Implementation variants:

- `BtrfsSnapshotter`
- `CopySnapshotter`
- (future) `ZfsSnapshotter`, `CsiSnapshotter`

---

## 7. State Layout

```text
state-store/
  engines/
    postgres/
      15/
        base/
        states/<uuid>/
      17/
        base/
        states/<uuid>/
  metadata/
    state.db
```

- Each state is immutable once created.
- Sandboxes use writable clones of states.

---

## 8. Snapshot Consistency Modes

### 8.1 Crash-Consistent (Default)

- Filesystem snapshot without stopping the DB.
- Relies on DB crash recovery on next start.
- Fastest and acceptable for most workflows.
- Default for interactive development and education sandboxes.

### 8.2 Clean Snapshot (Optional)

- DB container is stopped gracefully.
- Snapshot is taken.
- Container may be restarted if needed.

Used for:

- CI/CD
- strict reproducibility
- Can be requested by policy or per-run flag.

---

## 9. Sandbox Lifecycle

### 9.1 Creation

1. Select `(engine, version)`.
2. Select base state.
3. Clone base state into sandbox state.
4. Start container with sandbox state mounted.

### 9.2 Execution

- Liquibase or user commands run inside the sandbox.
- DB connections are proxied or exposed as needed.

### 9.3 Snapshotting

After a successful step:

1. Snapshot sandbox state into immutable state.
2. Register state in cache index.

### 9.4 Teardown and Cooldown

- After use, sandbox enters a cooldown period.
- If reused, container may be kept warm.
- Otherwise, container is stopped and sandbox destroyed.

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

- Docker + btrfs
- Postgres 15 & 17
- Local state store

### Phase 2

- Remote/shared cache
- ZFS / send-receive
- Pre-warmed pinned states

### Phase 3

- Kubernetes (CSI snapshots)
- MySQL engine adapter
- Cloud-scale eviction policies

---

## 12. Open Questions

- Minimum viable abstraction for volume handles across snapshotters?
- How aggressively to snapshot large seed steps?
- Default cooldown policy for sandboxes?
