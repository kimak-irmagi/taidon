# ADR 2026-02-01: Prepare kind variants and Liquibase planning

Status: Accepted
Date: 2026-02-01

## Decision Record

- Timestamp: 2026-02-01T20:45:00+00:00
- User: @Zlygo
- Agent: Codex (GPT-5)
- Question: How should `sqlrs prepare` support Liquibase locally while keeping a single prepare command and deterministic state reuse?
- Alternatives:
  - Add a separate command (`prepare:liquibase` or `prepare:lb`).
  - Keep a single `prepare` command with `prepare:lb` and forward Liquibase args.
  - Fingerprint inputs by hashing all input files and arguments.
  - Fingerprint inputs by Liquibase changeset content (via `updateSQL`).
- Decision:
  - Use a single `sqlrs prepare` command with `:<kind>` to select the variant (`prepare:psql`, `prepare:lb`).
  - Forward Liquibase arguments after `--` and filter unsafe options (connection args, non-update commands).
  - Plan via `liquibase updateSQL`, parse its output to build an ordered list of changesets and per-changeset SQL hashes.
  - Identify states by `prev_state_id` + ordered changesets (id/author/path + sql_hash).
  - Execute changesets one by one via `update-count --count=1`, snapshotting after each.
  - When running Liquibase in a container, mount only the local paths referenced by Liquibase args (including `--searchPath`) and rewrite them to `/sqlrs/mnt/pathN`.
- Rationale:
  - Preserves a single command shape while allowing multiple preparation backends.
  - Changeset-based fingerprints maximize cache reuse even when argument sets differ.
  - Parsing `updateSQL` avoids pre-traversing files and keeps Liquibase responsible for file resolution.
  - Per-changeset execution with snapshots yields fine-grained state reuse and deterministic planning.

## Context

- Liquibase must be supported locally without introducing a new top-level command.
- Users expect the Liquibase CLI to handle changelog formats and include resolution.
- Deterministic reuse requires a changeset-aware plan, not just file hashing.
