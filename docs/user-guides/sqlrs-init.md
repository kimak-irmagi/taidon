# sqlrs init — Workspace Initialization

## Overview

`sqlrs init` initializes or validates a **sqlrs workspace**.

A workspace is identified by the presence of a `.sqlrs/` directory.
Git repositories are **explicitly ignored**; git integration is the user's responsibility.

The command follows the _principle of least surprise_:

- it never silently creates nested workspaces,
- it never changes global state unless explicitly requested,
- it fails loudly on ambiguity.

---

## Workspace Detection Rules

### What counts as a workspace

A directory is considered a sqlrs workspace if:

- a `.sqlrs/` directory exists **in that directory**.

Inside `.sqlrs/`, the presence of a config file is validated, but **the directory itself is the primary marker**.

Expected layout (minimal):

```text
.sqlrs/
  config.yaml
```

Additional subdirectories MAY exist but are not required at init time.

### Parent workspace detection

Before initializing a workspace, `sqlrs init` scans **upwards** from the target directory:

- if a `.sqlrs/` directory is found in any parent,
- and the target directory itself does NOT contain `.sqlrs/`,

then a **parent workspace conflict** is detected.

---

## Default Behavior

```bash
sqlrs init
```

Equivalent to:

```bash
sqlrs init --workspace <current working directory>
```

Behavior:

1. If `<workspace>/.sqlrs/` exists:

   - validate structure
   - validate config (if present)
   - exit successfully (idempotent)

2. If `<workspace>/.sqlrs/` does NOT exist:
   - if a parent workspace exists → **error**
   - otherwise:
     - create `<workspace>/.sqlrs/`
     - create `<workspace>/.sqlrs/config.yaml` with defaults
     - exit successfully

---

## Configuration Format

- Local workspace config format: **YAML**
- Default file name: `.sqlrs/config.yaml`
- `sqlrs init` may write workspace config when flags require it (e.g., `--engine`, `--shared-cache`, `--wsl`)
- If config exists but cannot be parsed → treated as corruption

Global config is NOT created or modified by default.

---

## Flags

### `--workspace <path>`

Explicitly specify the workspace root.

All detection and initialization logic applies relative to this path.

### `--force`

Allow creation of a **nested workspace** even if a parent workspace exists.

This flag MUST be explicit.

### `--engine <path>`

Set the sqlrs engine reference in the local workspace config.

- Overrides built-in defaults
- Overrides global config (if any)
- Stored in `.sqlrs/config.yaml`
- If a **relative path** is provided **and** the current working directory is inside the workspace, the path is stored **relative to `.sqlrs/config.yaml`**.
- Otherwise (absolute path, or `sqlrs init` run outside the workspace via `--workspace`), the path is stored as an **absolute path**.

### `--shared-cache`

Enable usage of the global state cache for this workspace.

This flag only writes local configuration.
Global cache initialization is deferred.

### `--dry-run`

Do not create or modify any files.
Print intended actions.

### `--update`

Allow updating an existing workspace configuration.

- If `.sqlrs/` already exists, config updates are applied.
- Without `--update`, an existing workspace remains unchanged.
- If `.sqlrs/` does not exist, `--update` behaves like a normal init (creates workspace).
- If `config.yaml` is missing or corrupted, `--update` recreates it.
- If `--update` is used with `--wsl` and WSL init fails, the workspace config is left unchanged.

### `--wsl`

Enable WSL2 setup flow on Windows:

- validates WSL availability,
- resolves the target distro (default or `--distro`),
- ensures the WSL state dir is backed by btrfs (host VHDX + GPT + btrfs mount),
- installs a systemd mount unit inside the WSL distro so the btrfs mount is visible
  to Docker and all child processes,
- writes `engine.wsl.*` settings into `.sqlrs/config.yaml` (including mount metadata).

WSL+btrfs requires **systemd** inside the selected distro. If systemd is not enabled:

- with `--require`: init fails,
- without `--require`: init warns and falls back to non-WSL configuration.

After a successful WSL init, WSL must be restarted (`wsl.exe --shutdown`) so the
systemd unit is activated.

If `--require` is **not** set, WSL failures produce a warning and fallback to non-WSL configuration.

### `--store-size <N>GB`

Set the size of the **host VHDX** used for the WSL btrfs state store.

- Required suffix: `GB`
- Default: `100GB`

### `--reinit`

Recreate the WSL btrfs store from scratch (destructive).

Also disables and removes the WSL systemd mount unit before recreating the volume.

### `--distro <name>`

Use a specific WSL distro name. If omitted, the default distro is used (or a single available distro).

### `--require`

Require WSL+btrfs. If unavailable, `sqlrs init` fails (no fallback).

### `--no-start`

Do not start the WSL distro during init.

---

## Global State Interaction

`sqlrs init`:

- does NOT create global config
- does NOT repair global cache
- does NOT modify global engine state

If global resources are missing, a **hint** MAY be printed.

---

## Error Conditions and Exit Codes

| Condition                 | Message (summary)                   | Exit Code |
| ------------------------- | ----------------------------------- | --------- |
| Parent workspace detected | Refusing to create nested workspace | 2         |
| Config parse error        | Workspace config is corrupted       | 3         |
| Filesystem error          | Cannot create .sqlrs directory      | 4         |
| Invalid flags             | Invalid arguments                   | 64        |
| Unknown error             | Internal error                      | 1         |

---

## Examples

### Initialize a new workspace

```bash
sqlrs init
```

### Initialize explicitly at path

```bash
sqlrs init --workspace ./db-env
```

### Override engine path

```bash
sqlrs init --engine /opt/sqlrs/engine
```

### Force nested workspace (discouraged)

```bash
sqlrs init --force
```

### Initialize WSL (Windows)

```bash
sqlrs init --wsl
```

### Update an existing workspace config

```bash
sqlrs init --update --wsl
```

### Initialize WSL with explicit distro and no autostart

```bash
sqlrs init --wsl --distro Ubuntu-22.04 --no-start
```

---

## Design Rationale

- `.sqlrs/` directory is a strong, explicit workspace marker
- Parent workspace detection prevents accidental environment splits
- YAML chosen for human readability and ecosystem compatibility
- `init` is safe and conservative; repair and editing are separate concerns

---

## State Machine / Decision Table

This section defines the **formal decision logic** for `sqlrs init`.
It is intended to be normative for implementation.

### Definitions

- **Target path (T)** — path resolved from `--workspace` or `cwd`
- **Local marker** — directory `T/.sqlrs/`
- **Parent marker** — any `.sqlrs/` found strictly above `T`
- **Valid config** — `T/.sqlrs/config.yaml` exists and is parseable YAML

### High-level State Machine

```text
START
  |
  v
Resolve target path (T)
  |
  v
Check for parent marker
  |
  +-- found parent marker AND no local marker?
  |        |
  |        +-- --force? --> continue
  |        |
  |        +-- otherwise --> ERROR (nested workspace)
  |
  v
Check for local marker
  |
  +-- exists?
  |      |
  |      +-- validate config
  |      |       |
  |      |       +-- valid --> SUCCESS (idempotent)
  |      |       |
  |      |       +-- invalid --> ERROR (corrupt config)
  |
  +-- does not exist
         |
         +-- create .sqlrs/
         |
         +-- create config.yaml
         |
         +-- SUCCESS
```

### Decision Table

| #   | Parent `.sqlrs/` | Local `.sqlrs/` | `--force` | Config state | Result                         |
| --- | ---------------- | --------------- | --------- | ------------ | ------------------------------ |
| 1   | no               | no              | n/a       | n/a          | Create new workspace           |
| 2   | no               | yes             | n/a       | valid        | Success (idempotent)           |
| 3   | no               | yes             | n/a       | invalid      | Error: corrupt config          |
| 4   | yes              | no              | no        | n/a          | Error: nested workspace        |
| 5   | yes              | no              | yes       | n/a          | Create nested workspace        |
| 6   | yes              | yes             | n/a       | valid        | Success (local workspace wins) |
| 7   | yes              | yes             | n/a       | invalid      | Error: corrupt local config    |

### Notes and Invariants

- Local workspace **always has priority** over parent workspace.
- Nested workspaces are allowed **only** with `--force`.
- `sqlrs init` never silently switches target to a parent workspace.
- Any YAML parse error is treated as configuration corruption.
- No global state is created or modified in any state.

### Exit Code Mapping

| Result                    | Exit Code |
| ------------------------- | --------- |
| Success / idempotent      | 0         |
| Nested workspace conflict | 2         |
| Corrupt config            | 3         |
| Filesystem error          | 4         |
| Invalid arguments         | 64        |
| Internal error            | 1         |

---

## Implementation Notes (Non-normative)

- Parent marker detection MUST stop at filesystem root.
- Symlinks SHOULD be resolved before marker detection.
- All filesystem mutations SHOULD be atomic where possible.
- `--dry-run` follows the same state machine but skips mutations.
