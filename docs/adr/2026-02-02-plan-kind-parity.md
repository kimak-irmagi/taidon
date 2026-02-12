# ADR 0011: Plan command parity across prepare kinds

- **Timestamp:** 2026-02-02
- **GitHub user id:** @evilguest
- **Agent:** Codex (GPT-5)

## Question

Should every supported `prepare:<kind>` automatically have a matching `plan:<kind>` with identical arguments, differing only by `plan_only` execution semantics?

## Alternatives considered

1. Keep `plan:psql` as the only plan command and add other kinds ad-hoc later.
2. Introduce `plan:<kind>` for every supported `prepare:<kind>`, using the same argument handling and a `plan_only` execution path in the engine.

## Decision

Choose alternative 2: enforce `plan:<kind>` parity for every supported `prepare:<kind>`, with identical argument handling and a single plan-only execution path in the engine.

## Rationale

- Keeps CLI semantics consistent and predictable across kinds.
- Avoids duplicated/branchy plan-only logic in the engine by reusing the same planner and skipping execution.
- Enables documentation and UX to scale as new `prepare:<kind>` variants are added.
