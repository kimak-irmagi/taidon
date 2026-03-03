# ADR: Compact tree prefixes and IMAGE_ID shortening in CLI output

- Conversation timestamp: 2026-03-03T13:52:39+07:00
- GitHub user id: @evilguest
- Agent: GPT-5.2 (Codex CLI)

## Question

How should `sqlrs ls --states` and `sqlrs rm` present state hierarchies and Docker image identifiers in human-readable output?

## Alternatives considered

### Tree rendering

1. Keep `sqlrs ls --states` flat and keep the existing `sqlrs rm` tree style (`|--`, ``--`, 4-space indents).
2. Use the same `|--` / ``--` style in both `sqlrs ls --states` and `sqlrs rm`.
3. Use a compact tree prefix encoding with **one character per depth level** (`+`, `` ` ``, `|`, space) and use it consistently across commands.

### IMAGE_ID formatting

A. Print full `IMAGE_ID` values always.
B. Shorten digest-based references in human output by default, keeping full values behind `--long`.

## Decision

- Adopt alternative 3 for human output: compact tree prefixes in both `sqlrs ls --states` and `sqlrs rm`.
- Adopt IMAGE_ID alternative B: shorten digest-based `IMAGE_ID` values in human output unless `--long` is set.
- Document that `sqlrs` `IMAGE_ID` is typically comparable to Docker `RepoDigests`, not to the Docker `IMAGE ID` column.

## Rationale

- Deep hierarchies remain readable in narrow terminals (e.g., <100 columns).
- A single tree style across commands reduces cognitive load.
- Compact `IMAGE_ID` keeps tables readable while `--long` preserves full fidelity.
