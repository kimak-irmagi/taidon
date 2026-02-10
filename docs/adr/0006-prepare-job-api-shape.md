# ADR 0006: Prepare job API shape (async, kind-specific args)

Status: Accepted
Date: 2026-01-15

## Decision Record 1

- Timestamp: 2026-01-15
- User: @evilguest
- Agent: none
- Question: Which verb should the job API use to schedule a new job?
- Alternatives:
  - Use `POST /v1/prepare-jobs`
  - Use `PUT /v1/prepare-jobs/<new-job-id>` with client-generated job id
  - Use `PUT /v1/prepare-jobs/<new-job-id>` with a server-generated job id
- Decision: use the `POST` verb.
- Rationale: Even though the strict REST insists on using idempotence for all the
  modification operations, following this principle here would just complicate the
  API usage with little benefits. Client-generated IDs mean forcing us to use GUIDs
  that aren't much convenient to produce, store, and consume. Server-generated IDs
  mean implementing yet another endpoint that serves no purpose other than
  satisfying the purists.
  Crashing during the POST operation would mean an orphaned job. Jobs are
  lightweight and ephemeral by design, so this woudn't cause a long-term resource
  leak. Worst-case scenario is generating a bunch of garbage jobs.
  Since the job executions are idempotent, these garbage jobs wouldn't cause much
  harm - the extra ones would just immediately complete yielding the same state.
  The potential _instance_ pollution should be prevented by the design of the
  API behavior. This should take an additional discussion/design round.

### Context

- CLI can resolve defaults from config, but the engine should not assume implicit
  defaults.
- Future prepare kinds (Liquibase, etc.) need separate parameter sets without
  breaking compatibility.

## Decision Record 2

- Timestamp: 2026-01-15T16:30:57+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the prepare job API behave, and how should kind-specific
  arguments be modeled?
- Alternatives:
  - Make `POST /v1/prepare-jobs` synchronous and return `200 OK` with the final
    result.
  - Use `201 Created` and block until completion.
  - Keep a single flat request schema instead of kind-specific schemas.
  - Allow `image_id` to be optional with an engine-side default.
- Decision:
  `POST /v1/prepare-jobs` returns `202 Accepted` with a job reference and `Location`;
  clients poll `GET /v1/prepare-jobs/{job_id}` and/or stream
  `GET /v1/prepare-jobs/{job_id}/events`.
  The request schema is kind-specific via a discriminator (`prepare_kind`),
  with a `psql` schema today and room for future kinds.
  `image_id` is required in the engine request.
- Rationale: Prepare can be long-running, so async jobs avoid holding connections.
  A discriminator-based schema keeps kind-specific parameters extensible.
  The engine must know the base image explicitly.

### Context

- CLI can resolve defaults from config, but the engine should not assume implicit
  defaults.
- Future prepare kinds (Liquibase, etc.) need separate parameter sets without
  breaking compatibility.
