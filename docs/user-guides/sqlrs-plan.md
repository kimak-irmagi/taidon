# sqlrs plan

## Overview

`sqlrs plan` computes the execution plan for a deterministic preparation without
creating an instance or queueing a job. It reports which steps are already
cached and which steps would run during `prepare`.

---

## Command Syntax

```text
sqlrs plan:psql [--image <image-id>] [--] [psql-args...]
```

Where:

- `--image <image-id>` overrides the base Docker image.
- `psql-args` are passed to `psql` and fully describe how the state is produced.

If `--` is omitted, all remaining arguments are treated as `psql-args`.
To pass `psql` flags that would clash with sqlrs flags (for example `-v`),
use `--` explicitly.

---

## `plan:psql` Concept

`plan:psql` defines:

- how the database state would be constructed,
- which inputs participate in state identification,
- which execution steps would be reused from cache.

---

## psql Argument Handling

`plan:psql` aims to match `psql` semantics closely. All `psql-args` are passed
verbatim to `psql` with two enforced defaults for determinism:

- `-X` (ignore `~/.psqlrc`)
- `-v ON_ERROR_STOP=1`

If a user-provided argument conflicts with these enforced defaults, `plan`
fails with an error.

Connection arguments are rejected because sqlrs supplies the connection for
preparation:

- `-h`, `--host`
- `-p`, `--port`
- `-U`, `--username`
- `-d`, `--dbname`, `--database`

### SQL input sources

- `-f`, `--file <path>`: SQL script file (absolute paths are passed to the engine).
- `-c`, `--command <sql>`: inline SQL string.
- `-f -`: read SQL from stdin; sqlrs reads stdin and passes it to the engine.

All inputs above participate in plan identification.

---

## Base Image Selection

The base Docker image id is resolved in this order:

1. `--image <image-id>` command-line flag
2. Workspace config (`.sqlrs/config.yaml`, `dbms.image`)
3. Global config (`$XDG_CONFIG_HOME/sqlrs/config.yaml`, `dbms.image`)

The image id is treated as an opaque value and passed to Docker as-is.
If no image id can be resolved, `plan` fails.

Config key:

```yaml
dbms:
  image: postgres:17
```

When `-v/--verbose` is set, sqlrs prints the resolved image id and its source.

---

## Plan Tasks

`plan` returns an ordered list of tasks. The current task types are:

- `plan`: build the plan for the requested kind.
- `state_execute`: apply a change block on a base image or state.
- `prepare_instance`: create an ephemeral instance from the final state.

Each `state_execute` task includes:

- `task_hash`: hash of the change block.
- `output_state_id`: deterministic id derived from input id and `task_hash`.
- `cached`: whether the output state already exists.

If `cached` is true, the `prepare` executor can skip the execution step and
reuse the existing state.

Snapshotting is part of `state_execute`.

---

## Output

Use the global `--output` flag to select the format.

### JSON output

```json
{
  "prepare_kind": "psql",
  "image_id": "postgres:17",
  "prepare_args_normalized": "-X -v ON_ERROR_STOP=1 -f /abs/init.sql",
  "tasks": [
    {
      "task_id": "plan",
      "type": "plan",
      "planner_kind": "psql"
    },
    {
      "task_id": "execute-0",
      "type": "state_execute",
      "input": { "kind": "image", "id": "postgres:17" },
      "task_hash": "2f9c...e18",
      "output_state_id": "8a1c...b42",
      "cached": false
    },
    {
      "task_id": "prepare-instance",
      "type": "prepare_instance",
      "input": { "kind": "state", "id": "8a1c...b42" },
      "instance_mode": "ephemeral"
    }
  ]
}
```

### Human output

Human output prints a summary with the final state id and a task list.

---

## Examples

### Plan a psql prepare

```bash
sqlrs plan:psql -- -f ./init.sql
```

---

### Override base image

```bash
sqlrs plan:psql --image postgres:17 -- -f ./init.sql
```

---

### Use stdin

```bash
cat ./init.sql | sqlrs plan:psql -- -f -
```

---

## Guarantees

- `plan` never creates instances or states.
- `plan` is deterministic with respect to its inputs.
