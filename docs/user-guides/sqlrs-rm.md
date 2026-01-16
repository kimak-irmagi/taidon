# sqlrs rm

## Overview

`sqlrs rm` removes a single instance or state identified by an id prefix.

- Instances are removed directly (if allowed).
- States are removed only if they have no descendants, or when `--recurse` is set.
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
-f, --force     Ignore active connections
--dry-run       Show what would be deleted without making changes
```

---

## Id Resolution

`<id_prefix>` is treated as a prefix, not a full id.

Rules:

- Hex only, case-insensitive, minimum length 8.
- If multiple instances match, the id is ambiguous and the command fails.
- If multiple states match, the id is ambiguous and the command fails.
- If both a state and an instance match, the id is ambiguous and the command fails.
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

Flags:

- `--force` does not imply `--recurse`.
- The command always targets a single id prefix.

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
- `kind` is `state` or `instance`.
- `connections` is present for instance nodes.
- `blocked` is present only when deletion is not possible for that node.

Blocked reason codes:

- `active_connections`
- `has_descendants`
- `blocked_by_descendant`

---

## Exit Codes

- `0` - success, or noop (no matching id)
- `2` - invalid id prefix or ambiguous id
- `3` - internal error
- `4` - blocked by safety rules (active connections or missing `--recurse`)

---

## Examples

Remove an instance or state by id prefix:

```bash
sqlrs rm 6b6f3a8c
```

Remove a state and all its descendants:

```bash
sqlrs rm -r 6b6f3a8c
```

Force removal even with active connections:

```bash
sqlrs rm -f 6b6f3a8c
```

Preview what would be removed:

```bash
sqlrs rm -r --dry-run 6b6f3a8c
```
