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
sqlrs init [--wsl] [--distro <name>] [--require] [--no-start]
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

Success criteria:

- WSL is available (if `--wsl`).
- WSL distro resolved (explicit or default).
- btrfs state-store path is valid (see below).
- Config written (WSL mode/distro/stateDir).
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
- initialization (create loopback image + btrfs mount),
- re-initialization (explicit flag).

Additional flags can be added to control size/path/re-init behavior.

Current implementation details:

- Loopback image path: `~/.local/share/sqlrs/btrfs.img`
- Default image size: `10G`
- Mount point (state dir): `~/.local/state/sqlrs`
- Mount command uses `sudo` inside the distro (may prompt for a password).

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

- WSL state dir is inside Linux filesystem.
- Windows CLI reads it via `/mnt/...` path that maps to the WSL state dir.
- The file path is resolved through `wslpath` + interop when WSL is in use.

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

- default: `~/.local/state/sqlrs`,
- must be inside Linux filesystem (not `/mnt/c/...`).

### 4) WSL engine binary path (Linux path)

`engine.wsl.enginePath`

- default: auto (see binary provisioning below).

### 5) Snapshot backend (already implemented)

`snapshot.backend`

- `"auto" | "overlay" | "btrfs" | "copy"`.

When WSL is used, `"btrfs"` is expected.

---

## Engine Auto-Start (Windows CLI)

If WSL mode is active:

1. CLI resolves distro and state dir.
2. CLI checks btrfs for the **state-store** mount.
3. CLI starts engine inside WSL using `wsl.exe`:
   - engine binds loopback TCP
   - engine writes `engine.json` in WSL state dir
4. CLI reads `engine.json` and connects.

If WSL not used:

1. CLI spawns Windows `sqlrs-engine` as today.
2. CLI reads host `<StateDir>/engine.json` and connects.

---

## Open Decisions (need approval)

---

## Warning behavior

- Missing btrfs (when WSL is preferred) emits a **warning by default** and falls back to host engine.
- `--verbose` includes diagnostic details (detected distro, state dir, and probe failures).
