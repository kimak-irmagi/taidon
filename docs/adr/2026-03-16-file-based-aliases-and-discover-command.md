# 2026-03-16 File-Based Aliases and Discover Command

- Conversation timestamp: 2026-03-16T18:05:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: repo-tracked alias storage and resolution

### Question discussed

How should sqlrs store versioned workflow aliases for local repository use
without mixing them into local-only workspace configuration?

### Alternatives considered

1. Store aliases inside `.sqlrs/config.yaml` together with local engine and
   developer-specific workspace settings.
2. Store aliases under a dedicated repository tree such as `sqlrs/prepare/...`
   and `sqlrs/run/...`.
3. Store aliases as repo-tracked files addressed by workspace-relative stems and
   command-specific suffixes.

### Chosen solution

Adopt option 3.

Use repo-tracked alias files with command-specific suffixes:

- `*.prep.s9s.yaml` for prepare/plan aliases
- `*.run.s9s.yaml` for run aliases

Alias refs are workspace-root-relative stems:

- `sqlrs prepare chinook` -> `chinook.prep.s9s.yaml`
- `sqlrs run smoke` -> `smoke.run.s9s.yaml`
- `sqlrs prepare path/chinook` -> `path/chinook.prep.s9s.yaml`

An exact-file escape is available via a trailing `.`:

- `sqlrs prepare chinook.txt.` -> `chinook.txt`

### Brief rationale

This keeps versioned workflow recipes inside the repository, separates them from
local-only `.sqlrs/config.yaml`, avoids a second hardcoded tree like `sqlrs/`,
and gives IDE/schema tooling a stable filename pattern.

Workspace-root-relative stems avoid coupling the semantics to the current
working directory once the workspace is known.

## Decision Record 2: alias consumption syntax

### Question discussed

How should users invoke workflow aliases from the CLI?

### Alternatives considered

1. `sqlrs prepare --alias <name>`
2. `sqlrs prepare:alias <name>`
3. `sqlrs prepare <name>`

### Chosen solution

Adopt option 3.

The accepted alias-mode syntax is:

- `sqlrs plan <prepare-ref>`
- `sqlrs prepare <prepare-ref>`
- `sqlrs run <run-ref> --instance <id|name>`

Raw mode remains available and distinct:

- `sqlrs plan:<kind> ...`
- `sqlrs prepare:<kind> ...`
- `sqlrs run:<kind> ...`

### Brief rationale

If the alias definition already contains the tool kind and tool arguments, then
`--alias` and `:alias` add redundant syntax and make alias mode look like a flag
or pseudo-kind. Using the bare verb plus subject keeps alias mode symmetrical
and easy to read.

## Decision Record 3: discover as an umbrella command

### Question discussed

Should alias suggestion live under `sqlrs alias discover`, or should sqlrs use a
broader discovery command that can host multiple analyzers?

### Alternatives considered

1. `sqlrs alias discover`
2. `sqlrs discover ...` with alias discovery as one analyzer among others

### Chosen solution

Adopt option 2.

Use a dedicated verb:

```text
sqlrs discover [--aliases] [--gitignore] [--vscode] [--prepare-shaping]
```

`discover` is advisory and read-only by default. Execution commands do not
depend on prior discovery results.

### Brief rationale

The workspace can surface multiple useful improvement opportunities beyond alias
files alone. A general `discover` verb scales to those analyzers while keeping
execution semantics explicit and independent of heuristics.

## Decision Record 4: aliases and names remain separate entities

### Question discussed

How should repo-tracked aliases interact with runtime `names`?

### Alternatives considered

1. Treat aliases and names as the same conceptual namespace.
2. Allow aliases to implicitly become names or to resolve through the same
   lookup path.
3. Keep aliases and names separate, with optional metadata linkage only.

### Chosen solution

Adopt option 3.

- aliases are repo-tracked recipes
- names are runtime handles to materialized instances
- aliases and names do not share implicit lookup semantics
- a name may later expose optional metadata such as `source_alias`, but only for
  introspection

### Brief rationale

Aliases express how to build or run a workflow from repository content. Names
express which runtime instance is currently bound and available. Keeping them
separate minimizes surprise and preserves a clear distinction between versioned
project data and runtime state.

## Decision Record 5: updated M2 slice order

### Question discussed

How should the public/local M2 work be sliced after adopting explicit file-based
aliases and the broader `discover` command?

### Alternatives considered

1. Continue with the earlier repo-layout convention slice and add alias/discover
   later.
2. Start with `discover` first and let discovery semantics lead execution
   design.
3. Start with explicit file-based prepare aliases, then extend to run aliases,
   discovery, and later Git-aware slices.

### Chosen solution

Adopt option 3.

The approved slice order is:

1. file-based prepare aliases
2. run aliases and alias inspection
3. `discover --aliases`
4. generic discover analyzers
5. shared local input graph primitives
6. `sqlrs diff` path mode
7. Git ref execution baseline
8. provenance and cache explain

The selected first PR is the file-based prepare alias baseline for
`sqlrs plan <ref>` and `sqlrs prepare <ref>`.

### Brief rationale

Execution should be grounded in explicit repo-tracked recipes before sqlrs adds
advisory tooling around them. Discovery remains helpful, but it should not lead
or define the execution model. This sequence gives early user value while
preserving explicitness and leaving space for later Git-aware capabilities.

## Related documents

- `docs/user-guides/sqlrs-aliases.md`
- `docs/roadmap.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/adr/2026-03-09-git-diff-command-shape.md`
- `docs/adr/2026-03-16-m2-local-dx-slice-order.md`

## Contradiction check

`docs/adr/2026-03-16-m2-local-dx-slice-order.md` is superseded by this ADR and
should be marked obsolete.

This ADR does not change the accepted `sqlrs diff` command shape from
`2026-03-09-git-diff-command-shape.md`. It changes the local M2 execution model
that precedes the later Git-aware slices.
