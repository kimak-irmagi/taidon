# sqlrs rm

## Overview

`sqlrs rm` removes a single instance, state, or job identified by an id prefix
or a full job id.

- Instances are removed directly (if allowed).
- States are removed only if they have no descendants, or when `--recurse` is set.
- Jobs are removed directly (if allowed).
- The command is synchronous (no job id).

---

## Command Syntax

```text
sqlrs rm [OPTIONS] <id_prefix>
```

---

## Options

```text
-r, --recurse   Remove descendant states and instances
-f, --force     Ignore active connections / allow deleting active jobs
--dry-run       Show what would be deleted without making changes
```

---

## Id Resolution

`<id_prefix>` is treated as a prefix for states and instances. Job ids must be
provided in full.

Rules:

- State/instance prefixes are hex only, case-insensitive, minimum length 8.
- If multiple instances match, the id is ambiguous and the command fails.
- If multiple states match, the id is ambiguous and the command fails.
- If both a state and an instance match, the id is ambiguous and the command fails.
- Job ids are matched by exact id (no prefix matching).
- If a job id matches and a state/instance prefix also matches, the id is ambiguous.
- If nothing matches, the command prints a warning and exits 0 (noop).
- All ids in output are lowercased.

---

## Deletion Rules

Instances:

- An instance with active connections is blocked unless `--force` is set.

States:

- A state with descendants is blocked unless `--recurse` is set.
- When `--recurse` is set, all descendants are considered.
- If any descendant is blocked (for example, active connections without `--force`),
  the command deletes nothing.

Jobs:

- A job is blocked when at least one task has started unless `--force` is set.

Flags:

- `--force` does not imply `--recurse`.
- The command always targets a single id prefix or job id.

---

## Output (Human)

Human output is a tree:

- `deleted` or `would delete` for successful actions
- `blocked (<reason>)` for any node that cannot be deleted
- Instances include `connections=<n>`

Example (real deletion):

```text
state 6b6f... deleted
|-- instance 6b6f... deleted (connections=0)
`-- state 6b6f... deleted
```

Example (job deletion):

```text
job 0a1b... deleted
```

Example (`--dry-run`):

```text
state 6b6f... would delete
`-- instance 6b6f... blocked (active_connections) (connections=2)
```

If `--recurse` is not set, only the root node is shown.

---

## Output (JSON)

With `--output json`, the command prints a single JSON object:

```json
{
  "dry_run": true,
  "outcome": "blocked",
  "root": {
    "kind": "state",
    "id": "6b6f...",
    "children": [
      {
        "kind": "instance",
        "id": "6b6f...",
        "connections": 2,
        "blocked": "active_connections"
      }
    ]
  }
}
```

Rules:

- `dry_run` is `true` only when `--dry-run` is set.
- `outcome` is `deleted`, `would_delete`, or `blocked`.
- `kind` is `state`, `instance`, or `job`.
- `connections` is present for instance nodes.
- `blocked` is present only when deletion is not possible for that node.

Blocked reason codes:

- `active_connections`
- `has_descendants`
- `blocked_by_descendant`
- `active_tasks`

---

## Exit Codes

- `0` - success, or noop (no matching id)
- `2` - invalid id prefix or ambiguous id
- `3` - internal error
- `4` - blocked by safety rules (active connections, active tasks, or missing `--recurse`)

---

## Examples

Remove an instance or state by id prefix:

```bash
sqlrs rm 6b6f3a8c
```

Remove a job by id:

```bash
sqlrs rm 0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d
```

Remove a state and all its descendants:

```bash
sqlrs rm -r 6b6f3a8c
```

Force removal even with active connections:

```bash
sqlrs rm -f 6b6f3a8c
```

Force removal of a started job:

```bash
sqlrs rm -f 0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d
```

Preview what would be removed:

```bash
sqlrs rm -r --dry-run 6b6f3a8c
```
