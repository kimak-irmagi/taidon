# sqlrs aliases

## Overview

**Status: partially implemented.**

Current local CLI support includes:

- `sqlrs plan <prepare-ref>`
- `sqlrs prepare <prepare-ref>`
- `*.prep.s9s.yaml` prepare alias files
- exact-file escape via a trailing `.`

Approved next slice:

- `sqlrs run <run-ref> [--instance <id|name>]`
- mixed `prepare ... run ...` composite invocations across raw and alias modes
- explicit `sqlrs alias ...` management commands
- `sqlrs discover ...`
- alias-driven name binding flags such as `--name`

This document defines how sqlrs should use **repo-tracked alias files** for
common `plan`, `prepare`, and `run` workflows without mixing those definitions
into local-only workspace configuration.

The core principle is explicitness:

- alias files are versioned together with the scripts or changelogs they refer to;
- local workspace settings stay in `.sqlrs/config.yaml`;
- execution commands resolve only the alias ref the user asked for;
- discovery tooling may suggest improvements, but execution never depends on
  guesswork.

---

## Entities

sqlrs should keep these concepts separate:

- **prepare alias** - a repo-tracked recipe describing how to build a state
- **run alias** - a repo-tracked preset describing how to run a tool against an
  existing instance
- **name** - a runtime handle bound to a materialized instance

Aliases are part of the repository contract. Names are part of runtime state.

Aliases and names must not share resolution semantics or a common implicit
namespace.

---

## Alias Files

Two versioned alias-file classes are defined:

- `*.prep.s9s.yaml` - prepare/plan alias definition
- `*.run.s9s.yaml` - run alias definition

Examples:

```text
chinook.prep.s9s.yaml
app/main.prep.s9s.yaml
smoke.run.s9s.yaml
perf/pgbench.run.s9s.yaml
```

These files are expected to be committed to the repository. They are distinct
from `.sqlrs/config.yaml`, which remains local-only and developer-specific.

---

## Alias Reference Resolution

Alias refs are **current-working-directory-relative logical stems**.

Workspace discovery still matters, but only as a **boundary and config root**:

- alias refs are resolved from the caller's current working directory;
- the resolved alias file must still stay within the active workspace;
- execution does not silently rebase alias refs against the workspace root.

### Prepare / plan resolution

```text
sqlrs plan <ref>
sqlrs prepare <ref>
```

Resolution rule:

- `sqlrs prepare chinook` -> `<cwd>/chinook.prep.s9s.yaml`
- `sqlrs prepare path/chinook` -> `<cwd>/path/chinook.prep.s9s.yaml`
- if `pwd = <workspace>/examples/chinook`, then
  `sqlrs prepare ../chinook` -> `<workspace>/examples/chinook.prep.s9s.yaml`

### Run resolution

```text
sqlrs run <ref> [--instance <id|name>]
```

Resolution rule:

- `sqlrs run smoke` -> `<cwd>/smoke.run.s9s.yaml`
- `sqlrs run path/smoke` -> `<cwd>/path/smoke.run.s9s.yaml`

### Exact-file escape

If the ref ends with a trailing `.`, sqlrs disables suffix inference and uses
the path literally after stripping the final dot:

- `sqlrs prepare chinook.txt.` -> `<cwd>/chinook.txt`

This is an escape hatch for explicit files. It is not the primary happy path.

### Paths inside alias files

Alias-file payloads are resolved relative to the alias file itself.

Examples:

- if `examples/chinook.prep.s9s.yaml` contains `- -f` / `- chinook/prepare.sql`,
  then `chinook/prepare.sql` resolves from `examples/`, not from the caller's
  current working directory;
- if `db/app.prep.s9s.yaml` contains `- --changelog-file` / `- changelog.xml`,
  then `changelog.xml` resolves to `db/changelog.xml`.

This rule applies only to paths read from the alias file. Raw command stages
such as `prepare:psql -- -f queries.sql` or `run:psql -- -f queries.sql` keep
their normal current-working-directory-relative path semantics.

### Non-goals of alias resolution

Alias resolution must not:

- scan the workspace recursively looking for a matching basename;
- silently reinterpret the requested ref as workspace-root relative;
- fall back to heuristics when the requested ref is missing.

If the resolved file does not exist, the command fails explicitly.

---

## Execution Commands

Alias mode and raw mode are intentionally separate.

### Raw mode

Raw mode keeps the existing command families:

```text
sqlrs plan:psql ...
sqlrs prepare:lb ...
sqlrs run:pgbench ...
```

### Alias mode

Alias mode uses the bare verb plus an alias ref as the subject:

```text
sqlrs plan <prepare-ref>
sqlrs prepare <prepare-ref>
sqlrs run <run-ref> [--instance <id|name>]
```

Examples:

```bash
sqlrs plan chinook
sqlrs prepare chinook
sqlrs run smoke --instance dev
```

### Alias mode rules

- `plan <prepare-ref>` and `prepare <prepare-ref>` load a prepare alias file.
- `run <run-ref>` loads a run alias file.
- In alias mode, the tool kind and tool arguments come from the alias file.
- An alias stage does **not** accept inline tool arguments of its own.
- Only orchestration flags remain valid in alias mode:
  - for `prepare`: `--watch`, `--no-watch`
  - future follow-up for `prepare`: `--name`, later `--reuse`, `--fresh`,
    `--rebind`
  - for `run`: `--instance`

This keeps the alias file as the canonical recipe instead of turning it into a
partial preset layered with ad-hoc command-line overrides.

### Mixed composite `prepare ... run`

The approved next slice allows the normal two-stage `prepare ... run`
invocation to mix raw and alias modes freely:

```text
sqlrs prepare <prepare-ref> run <run-ref>
sqlrs prepare <prepare-ref> run:<kind> [--instance <id|name>] [-- <command>] [args...]
sqlrs prepare:<kind> ... run <run-ref>
sqlrs prepare:<kind> ... run:<kind> [--instance <id|name>] [-- <command>] [args...]
```

Examples:

```bash
sqlrs prepare chinook run:psql -- -f queries.sql
sqlrs prepare:psql -- -f prepare.sql run smoke
sqlrs prepare chinook run smoke
```

Composite rules:

- A composite invocation still contains exactly two stages: one `prepare`
  stage and one `run` stage.
- The `prepare` stage may use alias mode or raw mode.
- The `run` stage may use alias mode or raw mode.
- If a `run` stage follows `prepare` in the same invocation, the target
  instance is the instance produced by that `prepare`.
- In that composite case, `--instance` must not be used on the `run` stage;
  providing it is an explicit ambiguity error.
- `--no-watch` or interactive `detach` still skip the subsequent `run` stage,
  regardless of whether that stage is raw or alias-backed.

---

## Alias File Shape

The initial alias files should remain intentionally small and explicit.

### Prepare alias example

```yaml
kind: lb
image: postgres:17
args:
  - update
  - --changelog-file
  - config/liquibase/master.xml
```

### Run alias example

```yaml
kind: psql
args:
  - -f
  - queries/smoke.sql
```

Initial requirements:

- `kind` is required
- `args` is required and ordered
- `image` is allowed only for prepare aliases

Future metadata may be added, but the initial format should stay easy to read,
review, and generate.

---

## Alias Management Commands

sqlrs should expose explicit alias-management commands that operate on the
repo-tracked alias files:

```text
sqlrs alias ls
sqlrs alias show <ref>
sqlrs alias validate [<ref>]
```

Purpose:

- `ls` - list discovered alias refs from the workspace
- `show` - print the resolved alias definition
- `validate` - validate schema and command-specific constraints

These commands inspect or validate alias files. They do not create runtime
instances and they do not replace `names`.

---

## `sqlrs discover`

`discover` is a general advisory command family for workspace analysis.

It is broader than alias generation and should remain a **verb**, not a
subcommand under `alias`.

Initial command shape:

```text
sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]
```

### Intended analyzer roles

- `--aliases`
  - detect candidate prepare/run workflows and suggest alias files
- `--gitignore`
  - suggest repository hygiene changes such as ignoring `.sqlrs/`
- `--vscode`
  - suggest editor integration files such as recommended extensions
- `--prepare-shaping`
  - suggest decomposition opportunities that could improve cache reuse

### Discover rules

- `discover` is advisory and read-only by default
- execution commands never depend on `discover`
- alias resolution never falls back to discovery results
- analyzers may be added incrementally over time

`discover --aliases` is the first expected discover slice because it directly
supports the alias-file workflow.

---

## Interaction with Names

Names remain runtime handles pointing to materialized instances.

Current implementation note:

- prepare alias mode does not yet expose `--name`
- alias/name interaction is therefore still limited to the documented future
  design

Examples:

```bash
sqlrs prepare chinook --name dev
sqlrs run smoke --instance dev
```

In this flow:

- `chinook` is a prepare alias ref
- `smoke` is a run alias ref
- `dev` is a runtime name

Rules:

- aliases do not automatically create or reuse names unless the command
  explicitly asks for it;
- names do not replace alias refs;
- aliases and names do not share implicit lookup;
- a name may later expose optional metadata such as `source_alias`, but only for
  introspection.

---

## Rationale Summary

This design optimizes for low surprise:

- repo-tracked workflows are explicit files;
- local-only settings stay local;
- alias refs are deterministic and cwd-relative;
- file-bearing paths inside alias files stay local to the alias file;
- `discover` helps authors improve a workspace but never becomes execution-time
  magic;
- runtime handles (`names`) stay distinct from repo-tracked recipes.

---

## Examples

Prepare from an alias file:

```bash
sqlrs prepare chinook
```

Run a preset against a named instance:

```bash
sqlrs run smoke --instance dev
```

Inspect workspace alias files:

```bash
sqlrs alias ls
sqlrs alias show chinook
sqlrs alias validate
```

Advisory discovery:

```bash
sqlrs discover
sqlrs discover --aliases
sqlrs discover --prepare-shaping
```
