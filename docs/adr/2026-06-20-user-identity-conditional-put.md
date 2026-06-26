# 2026-06-20 User Identity Conditional PUT

- Conversation timestamp: 2026-06-20T12:23:10.9884205+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: use conditional PUT for identity-keyed user profile writes

### Question discussed

Should user profile creation use non-idempotent collection `POST` endpoints, or
should it use idempotent identity-keyed `PUT` endpoints with standard HTTP
preconditions?

### Alternatives considered

1. Use `POST /v1/users/register` for self-registration and `POST /v1/users`
   for administrator-created users.
2. Use `POST /v1/users` plus a separate idempotency key supplied by the client.
3. Use conditional `PUT` on identity-keyed resources:
   - `PUT /v1/users/me` for self-registration and current-user updates.
   - `PUT /v1/users/by-identity?provider=...&issuer=...&subject=...` for
     administrator-created users and administrator updates.

### Chosen solution

Adopt option 3.

`PUT /v1/users/me` is the self-registration endpoint. The server derives the
target external identity from the validated bearer token and uses
`provider + issuer + subject` as the natural uniqueness key. Clients send
`If-None-Match: *` for create-only self-registration and `If-Match: <etag>` for
updates to an existing current-user profile.

`PUT /v1/users/by-identity` is the administrator endpoint for another external
identity. The identity tuple is supplied as query parameters because `issuer`
is commonly a URL and would require awkward path encoding or an additional
opaque key if placed in the path. The server still treats the tuple as the
resource key and enforces uniqueness over `provider + issuer + subject`.

Requests must include exactly one of `If-None-Match` or `If-Match`. Missing
preconditions fail with `428 Precondition Required`, conflicting preconditions
fail with `400 Bad Request`, and failed create-only or stale update
preconditions fail with `412 Precondition Failed`.

### Brief rationale

The external identity tuple is a natural idempotency key for user profile
creation. Conditional `PUT` lets clients safely retry after network failures,
uses standard HTTP semantics for create-vs-update intent, and prevents repeated
registration of the same external identity as a new user entity. Keeping
self-registration at `/v1/users/me` matches the current authenticated principal
without requiring the client to echo identity claims that the server must
derive from the bearer token anyway.

## Related documents

- `docs/api-guides/sqlrs-engine.openapi.yaml`
- `docs/user-guides/sqlrs-users-orgs.md`
- `docs/adr/2026-06-20-user-registration-command-split.md`

## Contradiction check

No existing ADR was marked obsolete. Existing ADRs do not define the user
profile creation API shape.
