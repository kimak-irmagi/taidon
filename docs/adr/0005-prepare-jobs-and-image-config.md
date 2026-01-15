# ADR 0005: Prepare jobs naming and image config key

Status: Accepted
Date: 2026-01-15

## Decision Record

- Timestamp: 2026-01-15T14:26:46+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the prepare job resource be named, and where should the base DBMS image be configured?
- Alternatives:
  - Use `task` instead of `job` for the prepare resource.
  - Put `prepare_kind` in the URL path (e.g., `/v1/prepare:psql`) instead of the request body.
  - Use `engine.image` as the config key for base images.
- Decision: Use `prepare-jobs` as the resource name (e.g., `POST /v1/prepare-jobs`) with `prepare_kind` in the request body, and use `dbms.image` as the config key for the base Docker image. The CLI `--image` flag overrides config and `-v/--verbose` prints the resolved image id and its source.
- Rationale: "Job" matches long-running prepare semantics, keeps kind as a parameter rather than a namespace, and `dbms.image` avoids confusion with the sqlrs-engine container image while remaining clear to users.

## Context

- `prepare` is a long-running operation that should be modeled as a job resource.
- The system must allow base image selection from both CLI and config.
- Future engine distribution as a container image makes `engine.image` ambiguous.
