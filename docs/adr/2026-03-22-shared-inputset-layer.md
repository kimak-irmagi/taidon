# 2026-03-22 Shared InputSet Layer

- Conversation timestamp: 2026-03-22T01:12:50.1633646+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question discussed

How should `sqlrs` avoid semantic drift between `prepare`, `plan`, `run`,
`diff`, and `alias check` when they all need to understand file-bearing
arguments and the resulting file-set closure?

## Alternatives considered

1. Keep separate helpers in `internal/app`, `internal/diff`, and
   `internal/alias`, then manually align their behavior over time.
2. Treat the new `diff` builders as the main implementation and make the
   execution and alias-check paths reuse `internal/diff`.
3. Introduce a shared CLI-side input-set layer, organized by **tool kind**
   (`psql`, `liquibase`, `pgbench`), which owns parsing, host-path binding,
   closure collection, and consumer-specific projection.

## Chosen solution

Adopt alternative 3.

`sqlrs` will gain a shared CLI-side input-set layer that is the single source
of truth for file-bearing semantics. The layer is organized by tool kind rather
than by top-level command:

- `plan:psql`, `prepare:psql`, and `run:psql` share one `psql` component;
- `plan:lb` and `prepare:lb` share one `liquibase` component;
- `run:pgbench` gets its own `pgbench` component.

The shared pipeline is:

```text
raw args
-> kind-specific parse
-> host-path binding through a resolver
-> file-set collection through a filesystem abstraction
-> consumer-specific projection
```

Consumers such as `prepare`, `plan`, `run`, `diff`, and `alias check` keep
their own orchestration and rendering, but they do not maintain separate
parsers/builders for the same tool kind.

## Brief rationale

The existing split across `internal/app`, `internal/diff`, and `internal/alias`
creates unavoidable semantic drift: the same file-bearing syntax is parsed and
interpreted differently in different command families.

A shared input-set layer solves the real architectural problem instead of
patching each consumer manually:

- one source of truth for `psql` / Liquibase / `pgbench` file semantics;
- explicit separation between host-path semantics and runtime-specific path
  projection;
- reuse across current consumers (`prepare`, `plan`, `run`, `diff`, `alias check`)
  and later consumers (`discover`, provenance, cache explain).

## Related documents

- `docs/architecture/inputset-component-structure.md`
- `docs/architecture/cli-component-structure.md`
- `docs/architecture/diff-component-structure.md`
- `docs/architecture/alias-inspection-component-structure.md`
- `docs/architecture/m2-local-developer-experience-plan.md`
- `docs/adr/2026-03-23-psql-include-base-semantics.md`

## Contradiction check

No direct contradictions were found in existing ADRs.

This ADR complements, but does not replace, the path-base decisions in
[`2026-03-19-alias-path-resolution-bases.md`](2026-03-19-alias-path-resolution-bases.md).
