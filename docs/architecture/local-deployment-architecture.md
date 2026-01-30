# Local Deployment Architecture (sqlrs)

This document describes how `sqlrs` is deployed and executed on a **developer workstation** in the MVP.

It focuses on:

- thin CLI design
- ephemeral engine process
- interaction with Docker and psql

This document intentionally avoids repeating script-runner details; Liquibase integration
is planned and covered in [`liquibase-integration.md`](liquibase-integration.md).
Engine internals are detailed in [`engine-internals.md`](engine-internals.md).
Team/Cloud variant is covered in [`shared-deployment-architecture.md`](shared-deployment-architecture.md).

---

## 1. Goals

- Fast CLI startup
- Minimal permanent footprint on the host system
- Cross-platform operation (Linux, macOS, Windows via WSL2)
- Clear separation between user-facing CLI and heavy runtime logic
- Easy evolution toward a persistent daemon or team/cloud deployment

---

## 2. High-Level Topology (MVP)

```mermaid
flowchart LR
  U[User]
  CLI["sqlrs CLI"]
  ENG["sqlrs engine process"]
  DOCKER["Docker Engine"]
  DB["DB Container"]
  STATE["State dir (engine.json)"]
  STORE["state store"]

  U --> CLI
  CLI -->|spawn / connect| ENG
  ENG --> DOCKER
  DOCKER --> DB
  ENG --> STORE
  CLI -. discovery .-> STATE
```

---

## 3. sqlrs CLI

### 3.1 Responsibilities

- Parse user commands and flags
- Interact with local filesystem (project config, paths)
- Discover or spawn a local engine process
- Communicate with engine via HTTP over loopback or Unix socket
- Execute `run` commands locally against a prepared instance/instance
- Exit immediately after command completion
- Optional: pre-flight checks (Docker reachable, state store writable), show engine endpoint/version for diagnostics

The CLI is intentionally **thin** and stateless.

### 3.2 Non-Responsibilities

- No Docker orchestration logic
- No snapshotting logic
- No direct script execution

---

## 4. Engine Process (Ephemeral)

### 4.1 Characteristics

- Started on-demand by the CLI
- Runs as a child process (not a system daemon)
- Listens on a local endpoint (loopback or socket)
- Manages runtime state while active

### 4.2 Responsibilities

- Docker container orchestration
- Snapshotting and state management
- Cache rewind and eviction
- Script execution via container `psql`
- Connection / proxy layer (when needed)
- Connection tracking for TTL and safe deletion
- IPC/API for CLI and future IDE integrations
- Prepare planning/execution; does not execute `run` commands
- Ephemeral instance creation for prepare

### 4.3 Lifecycle

- Spawned when required
- May persist for a short TTL after last request
- Terminates automatically when idle
- Writes its endpoint/lock and auth token into `<StateDir>/engine.json` to allow subsequent CLI invocations to reuse the same process

This avoids permanent background services in MVP.

---

## 5. IPC: CLI <-> Engine

- **Transport/Protocol**: REST over HTTP; loopback-only. Unix domain socket on Linux/macOS; TCP loopback on Windows host with WSL forwarding. No TLS in local mode.
- **Endpoint discovery**:
  - CLI checks `TAIDON_ENGINE_ADDR` env var.
  - Else reads `<StateDir>/engine.json` (contains endpoint, socket path / TCP port, PID, instanceId, auth token).
  - If not found or stale, CLI spawns a new engine; the engine writes `engine.json` when ready.
- **Security**: deny non-loopback binds; require auth token for non-health endpoints; rely on file perms (UDS) or loopback firewall; engine refuses connections from non-local addresses.
- **Versioning**: CLI sends its version; engine rejects incompatible major; CLI may suggest upgrade.

Key engine endpoints (logical):

- `POST /v1/prepare-jobs` - start prepare job (plan/execute steps, snapshot states, create instance); `plan_only` computes tasks only
- `GET /v1/prepare-jobs/{jobId}` - job status
- `GET /v1/prepare-jobs/{jobId}/events` - job event stream (NDJSON)
- `GET /v1/config` / `PATCH /v1/config` - server configuration read/write
- `GET /v1/config/schema` - configuration schema
- list names/instances/states (JSON array or NDJSON via `Accept`)
- `GET /v1/names/{name}` - read name binding
- `GET /v1/instances/{instanceId}` - read instance (supports name alias with 307 redirect to the canonical id URL when resolved by name)
- `DELETE /v1/instances/{instanceId}` - delete instance (idempotent; supports dry-run)
- `GET /v1/states/{stateId}` - read state
- `DELETE /v1/states/{stateId}` - delete state (idempotent; supports recurse/force/dry-run)
- `POST /snapshots` - manual snapshot
- `GET /cache/{key}` - cache lookup
- `POST /engine/shutdown` - optional graceful stop

### 5.1 Long-running operations: async jobs, sync CLI

- Engine handles prepare as asynchronous jobs; `POST /v1/prepare-jobs` returns `201 Created` with a job id.
- CLI immediately watches the job via status/events and exits when it reaches a terminal state.
- CLI currently has no detach mode; `--watch/--no-watch` is a future extension.

---

## 6. Interaction with psql

The engine executes `psql` inside the DB container via `docker exec`.
When file inputs are present, it bind-mounts the scripts root read-only and
rewrites file arguments to the container path.

Liquibase integration is planned; provider selection details live in
[`liquibase-integration.md`](liquibase-integration.md).

---

## 6. Interaction with Docker

- Docker is required in MVP
- Engine controls DB containers and executes `psql` via `docker exec`
- All persistent data directories are mounted from host-managed storage
- Engine validates Docker availability on start; CLI surfaces actionable errors if missing/denied

On Windows:

- Docker runs inside WSL2
- State store lives inside the Linux filesystem (btrfs loopback volume)
- `sqlrs init --wsl` installs a systemd mount unit inside the selected distro
  so the btrfs volume is visible to Docker and all child processes
- Engine verifies the mount is active before touching the store

---

## 7. Windows / WSL2 Considerations

- Engine and snapshotter run inside WSL2
- CLI may run on Windows host or inside WSL2
- Communication via localhost forwarding
- Engine writes `engine.json` to the Windows state directory and receives that path via `/mnt/...`
- Engine verifies the systemd mount for `SQLRS_STATE_STORE` at startup
- Snapshot backend may fall back to copy-based strategy

---

## 8. Evolution Path

### Phase 1 (MVP)

- Ephemeral engine process
- Thin CLI
- Local-only deployment

### Phase 2

- Optional persistent local daemon (`sqlrsd`)
- Warm instance reuse
- IDE integrations

### Phase 3

- Team-shared engine
- Remote cache
- Cloud-hosted control plane

---

## 9. Non-Goals

- System-wide background service by default
- OS-specific installers or service managers
- Deep Liquibase embedding

---

## 10. Open Questions

- Unix socket vs TCP loopback as default IPC?
- Default engine TTL after last command?
- Should CLI auto-upgrade engine binary?
