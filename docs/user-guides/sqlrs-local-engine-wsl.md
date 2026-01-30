# sqlrs local engine: WSL2 auto-start design (Windows)

## Goal

Provide a Windows workflow where:

- the **engine runs inside WSL2** (Linux),
- the **CLI can stay on the Windows host**,
- the CLI auto-connects to an existing engine or **auto-starts** it,
- WSL2 is **optional** (fallback to Windows host engine if WSL2 is unavailable),
- **btrfs is required** to use the WSL2 engine path (block CoW).

This document defines **CLI behavior**, **discovery rules**, and **configuration**.

---

## Terms

- **Host CLI**: `sqlrs` running on Windows.
- **WSL engine**: `sqlrs-engine` running inside WSL2.
- **Host engine**: `sqlrs-engine` running on Windows without WSL2.
- **SQLRS_STATE_HOME (host)**: `%APPDATA%\\sqlrs` (engine.json lives here).
- **SQLRS_STATE_STORE (host)**: `%LOCALAPPDATA%\\sqlrs\\store` (VHDX lives here).

---

## Behavior Summary

1. CLI first tries to connect to an existing engine (via `engine.json`).
2. If not found or stale, CLI attempts to auto-start the engine.
3. On Windows:
   - Prefer **WSL2 engine** when WSL2 + btrfs are available.
   - Otherwise **fallback to host engine** (copy snapshots).
4. If config **forces WSL2+btrfs** and not available, **fail** (no fallback).

---

## Proposed CLI flow (Windows)

### `sqlrs init` (local setup)

Purpose: one-time setup of WSL2 + btrfs + config for local engine auto-start.

Proposed syntax:

```text
sqlrs init [--wsl] [--distro <name>] [--require] [--no-start] [--store-size <N>GB] [--reinit]
```

Behavior:

- `--wsl`:
  - Enable WSL2 setup flow (default on Windows).
- `--distro <name>`:
  - Use specific WSL distro; default = WSL default distro.
- `--require`:
  - Set `engine.wsl.mode = "required"` in config.
  - Fail if WSL+btrfs is unavailable.
- `--no-start`:
  - Do not start engine after init.
  - Still validates prerequisites and writes config if successful.
- `--store-size <N>GB`:
  - Size of the host VHDX used as the WSL btrfs store.
  - Suffix `GB` is required.
- `--reinit`:
  - Recreate the VHDX + partition from scratch.
  - Use only when you are ok with data loss in the state store.

Success criteria:

- WSL is available (if `--wsl`).
- WSL distro resolved (explicit or default).
- btrfs state-store path is valid (see below).
- Config written (WSL mode/distro/stateDir/storePath + mount metadata).
- Optional: engine auto-start (unless `--no-start`).

---

## Decisions (approved)

### A) Engine binary provisioning (WSL)

The Windows local bundle **includes**:

- Windows CLI binary
- Windows engine binary
- Linux engine binary (same CPU arch)

`sqlrs init` copies the Linux engine binary into the WSL distro on first run.

### B) btrfs volume lifecycle

`sqlrs init` is responsible for:

- validation (existing volume),
- initialization (create host VHDX + GPT + btrfs mount),
- re-initialization (explicit flag).

Current implementation details:

- Host VHDX path: `%LOCALAPPDATA%\\sqlrs\\store\\btrfs.vhdx`
- Default image size: `100GB`
- WSL mount point (state dir): `~/.local/state/sqlrs/store`
- Mount command uses `wsl.exe -u root` (no `sudo`).

Initialization steps:

1. Create a **dynamic VHDX** on the host with the requested size.
2. Create a **GPT partition** that consumes the full disk.
3. Attach the VHDX to the selected WSL distro as **bare**.
4. Ensure `btrfs-progs` is installed inside the distro.
5. Format the partition as **btrfs**.
6. Mount the partition at `engine.wsl.stateDir` (inside WSL) using `-t btrfs`.
7. Verify mount via `findmnt -T <stateDir>`.
8. Create btrfs subvolumes in the mount root:
   - `@instances`
   - `@states`
9. `chown -R <user>:<group>` for the mount root so the engine can create `run/`.

### C) WSL auto-start

WSL distro is **auto-started by default**; `--no-start` disables it.

---

## Discovery and Connection

### Engine discovery

Order:

1. `TAIDON_ENGINE_ADDR` env var.
2. `<StateDir>/engine.json` (host path).
3. WSL `engine.json` (see WSL path rules below).

If found:

- verify process is alive (PID if local),
- verify endpoint responds to `/v1/health`,
- verify auth token matches.

If stale or missing: spawn engine.

### WSL engine.json path

- Engine writes `engine.json` to the **host** SQLRS_STATE_HOME directory
  (`%APPDATA%\\sqlrs`), but the CLI passes it to the WSL engine via a `/mnt/...`
  path (Windows host path translated to the WSL mount).

---

## WSL2 Selection Rules (Windows)

### Default (auto)

Use WSL engine when all of the following are true:

- WSL is available,
- the configured WSL distro exists and is running or can be started,
- the WSL state-store path is on **btrfs**.

Otherwise, fallback to host engine.

### Forced WSL mode

If config says `engine.wsl.mode = "required"`:

- WSL must be available,
- btrfs must be present.

If any condition fails: **error** (no fallback).

---

## Configuration (proposed)

### 1) WSL mode

`engine.wsl.mode`

- `"auto"` (default): try WSL+btrfs, else fallback to host engine.
- `"required"`: fail if WSL+btrfs is unavailable.
- `"off"`: skip WSL, always use host engine.

### 2) WSL distro selection

`engine.wsl.distro`

- empty: auto-pick the default WSL distro,
- set: explicit distro name.

### 3) WSL state dir (Linux path)

`engine.wsl.stateDir`

- default: `~/.local/state/sqlrs/store`,
- must be inside Linux filesystem (not `/mnt/c/...`).

### 4) WSL engine binary path (Linux path)

`engine.wsl.enginePath`

- default: auto (see binary provisioning below).

### 4.1 Orchestrator daemon path (host)

`orchestrator.daemonPath` points to the **linux** engine binary in the host bundle.
When launching inside WSL, the CLI converts this host path to a `/mnt/...` path
and passes it to `wsl.exe` to start the engine.

### 5) Host store path (Windows)

`engine.storePath`

- host path to the VHDX backing the WSL state store,
- default: `%LOCALAPPDATA%\\sqlrs\\store\\btrfs.vhdx`.

### 6) WSL mount metadata (Linux)

`engine.wsl.mount.device` and `engine.wsl.mount.fstype`

- recorded by `sqlrs init` after attaching/formatting the VHDX,
- passed by the CLI to the engine at startup,
- the engine always mounts to `SQLRS_STATE_STORE` (the target is not configurable).

### 6) Snapshot backend (auto by FS)

Engine selects snapshotter by **filesystem** of `SQLRS_STATE_STORE`:

- btrfs → btrfs snapshotter
- zfs dataset mount point (future) → zfs snapshotter
- otherwise → fallback copy/reflink

---

## Engine Auto-Start (Windows CLI)

If WSL mode is active:

1. CLI resolves distro and state dir.
2. CLI checks btrfs for the **state-store** mount.
3. CLI starts engine inside WSL using `wsl.exe`:
   - engine binds loopback TCP
   - engine writes `engine.json` under the **host** `SQLRS_STATE_HOME` path,
     passed as `/mnt/...` via `--write-engine-json`.
4. CLI reads `engine.json` and connects.

If WSL not used:

1. CLI spawns Windows `sqlrs-engine` as today.
2. CLI reads host `<StateDir>/engine.json` and connects.

### Engine mount check (WSL)

On startup, the engine ensures the btrfs device is mounted to `SQLRS_STATE_STORE`:

1. Read `engine.wsl.mount.*` from workspace config (CLI) and pass via env:
   - `SQLRS_WSL_MOUNT_DEVICE`
   - `SQLRS_WSL_MOUNT_FSTYPE`
2. Check mount via `findmnt -T $SQLRS_STATE_STORE`.
3. If missing, mount `device` using `-t <fstype>`.
4. Verify mount before touching the state store.

---

## Open Decisions (need approval)

---

## Warning behavior

- Missing btrfs (when WSL is preferred) emits a **warning by default** and falls back to host engine.
- `--verbose` includes diagnostic details (detected distro, state dir, and probe failures).
