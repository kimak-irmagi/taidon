# sqlrs ls

## Overview

`sqlrs ls` lists objects managed by sqlrs.

sqlrs manages five object types:

- **states** - immutable database states produced by `prepare:psql`
- **instances** - mutable database instances derived from states
- **names** - stable user-defined handles pointing to instances (and bound to a state fingerprint)
- **jobs** - prepare job executions and their lifecycle
- **tasks** - queued or running steps belonging to jobs

`ls` is a pure introspection command: it does not create, modify, or delete anything.

---

## Command Syntax

```text
sqlrs ls [OPTIONS]
```

### Object selectors (what to list)

```bash
--instances, -i   List instances
--states,    -s   List states
--names,     -n   List names
--jobs,     -j   List jobs
--tasks,    -t   List tasks
--all            Equivalent to -i -s -n --jobs --tasks
```

### Filtering and formatting

```text
--quiet          Suppress headers and explanatory text (table still printed)
--no-header      Do not print table header (human output)
--long           Show full ids in human output (default is 12 chars)
```

### Optional filters

```text
--name <name>            Filter by name (names/instances)
--instance <instance_id> Filter by instance id (full or hex prefix)
--state <state_id>       Filter by state id (full or hex prefix)
--job <job_id>           Filter by job id (jobs/tasks)
--kind <prepare_kind>    Filter by prepare kind (states)
--image <image id>       Filter by the base image
```

> Note: filters apply after object selection. If no selector flags are provided,
> defaults apply (see below).

### ID matching

`--instance` and `--state` accept full ids or hex prefixes (8+ characters).
Prefix matching is case-insensitive and resolves to a single id:

- If the prefix matches exactly one id, that id is used.
- If the prefix matches multiple ids, `ls` fails with an error.
- If the prefix matches nothing, the result is an empty list.

`--job` requires a full job id (no prefix matching).

---

## Default Behavior

If no object selector flags are given, `sqlrs ls` lists:

- `--names` and `--instances`

Rationale: these are the objects users interact with most often. `--states` can be
large (cache), and jobs/tasks are operational details, so they are shown explicitly.

---

## Output (Human-Readable)

By default, `sqlrs ls` prints a table per selected object type, in this order:

1. Names
2. Instances
3. States
4. Jobs
5. Tasks

Each section begins with a one-line title (suppressed by `--quiet`).
When `--quiet` is set and multiple sections are printed, sections are separated by a blank line.

IDs are printed in lowercase. By default, human output shortens ids to 12
characters. Use `--long` to print full ids.

### Names table

Columns:

- `NAME`
- `INSTANCE_ID`
- `IMAGE_ID`
- `STATE_ID` (or fingerprint-derived id)
- `STATUS` (active / expired / missing)
- `LAST_USED` (optional; if tracked)

Meaning:

- `STATUS=missing` indicates the name exists but its instance was purged/removed.
  The binding (name → state fingerprint) still exists.

### Instances table

Columns:

- `INSTANCE_ID`
- `IMAGE_ID`
- `STATE_ID`
- `NAME` (empty if ephemeral)
- `CREATED`
- `EXPIRES`
- `STATUS` (active / expired / orphaned)

Meaning:

- `STATUS=orphaned` indicates an instance exists without an attached name.

### States table

Columns:

- `STATE_ID`
- `IMAGE_ID`
- `PREPARE_KIND`
- `PREPARE_ARGS` (short, normalized)
- `CREATED`
- `SIZE`
- `REFCOUNT` (number of instances referencing this state)

### Jobs table

Columns:

- `JOB_ID`
- `STATUS` (queued / running / succeeded / failed / cancelled)
- `PREPARE_KIND`
- `IMAGE_ID`
- `PLAN_ONLY` (true / false)
- `CREATED`
- `STARTED`
- `FINISHED`

### Tasks table

Columns:

- `TASK_ID`
- `JOB_ID`
- `TYPE`
- `STATUS` (queued / running / succeeded / failed)
- `INPUT` (kind:id)
- `OUTPUT_STATE_ID` (state_execute only)
- `CACHED` (state_execute only)

---

## Output (JSON)

With the global `--output json` option, `sqlrs ls` prints a single JSON object:

```json
{
  "names": [ ... ],
  "instances": [ ... ],
  "states": [ ... ],
  "jobs": [ ... ],
  "tasks": [ ... ]
}
```

- Arrays are present only for the selected object types.
- Each element is a stable schema suitable for CI tooling.
- JSON output always uses full ids (no truncation).

Recommended fields:

### Name object

- `name`
- `instance_id` (nullable)
- `image_id`
- `state_id`
- `state_fingerprint`
- `status` (`active` | `missing` | `expired`)
- `last_used_at` (optional)

### Instance object

- `instance_id`
- `image_id`
- `state_id`
- `name` (nullable)
- `created_at`
- `expires_at` (nullable)
- `status` (`active` | `expired` | `orphaned`)

### State object

- `state_id`
- `image_id`
- `prepare_kind`
- `prepare_args_normalized`
- `created_at`
- `size_bytes` (optional)
- `refcount`

### Job object

- `job_id`
- `status` (`queued` | `running` | `succeeded` | `failed` | `cancelled`)
- `prepare_kind`
- `image_id`
- `plan_only`
- `created_at`
- `started_at` (nullable)
- `finished_at` (nullable)

### Task object

- `task_id`
- `job_id`
- `type`
- `status` (`queued` | `running` | `succeeded` | `failed`)
- `input` (object with `kind` and `id`)
- `task_hash` (optional)
- `output_state_id` (optional)
- `cached` (optional)
- `instance_mode` (optional)

---

## Exit Codes

- `0` — success (even if no objects match; empty lists are valid)
- `2` — invalid flags or invalid filter combinations
- `3` — internal error (storage backend unavailable, corrupted metadata, etc.)

---

## Examples

List active names and instances (default):

```bash
sqlrs ls
```

List everything:

```bash
sqlrs ls --all
```

List states only:

```bash
sqlrs ls -s
```

List instances derived from a specific state:

```bash
sqlrs ls -i --state <state_id>
```

List instances by state id prefix:

```bash
sqlrs ls -i --state deadbeef
```

List a single name entry:

```bash
sqlrs ls -n --name devdb
```

List jobs:

```bash
sqlrs ls --jobs
```

List tasks for a job:

```bash
sqlrs ls --tasks --job <job_id>
```

Machine-readable output for CI:

```bash
sqlrs ls --all --output json
```

---

## Notes and Design Rationale

### Why separate `names` from `instances`?

- Names are the *stable handles* users remember.
- Instances are the *actual mutable databases* that may be ephemeral, expiring, or renamed.
- Keeping both visible makes lifecycle debugging straightforward.

### Why not list states by default?

States are a cache of build artifacts. In shared mode, the cache can be large and
not directly user-relevant. Showing it explicitly avoids surprising output volume.

### Missing instance behind a name

`ls` intentionally reports `STATUS=missing` for names whose instance was purged.
