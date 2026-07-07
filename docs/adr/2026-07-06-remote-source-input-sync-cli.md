# 2026-07-06 Remote Source Input Sync in CLI

- Conversation timestamp: 2026-07-06T00:00:00+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Question Discussed

How should the Taidon CLI make local source files available to a remote gateway
after the server-side OpenAPI contract added `source_manifest`,
`source_inputs_missing`, and source blob upload support?

## Alternatives Considered

1. Add a separate explicit source-sync command that users run before prepare.
2. Upload a complete workspace snapshot before each remote prepare request.
3. Let the CLI compute the full format-specific transitive closure and upload it
   before submission.
4. Make source sync automatic for remote profiles: send a source manifest, let
   the server request missing hashes/listings/blobs, upload only requested
   blobs, and retry within a bounded loop.
5. Keep remote file-bearing prepare requests unsupported until a future artifact
   upload model exists.

## Decision

Use option 4.

Remote source sync is automatic profile behavior for `remote` profiles and does
not change command syntax. It can be disabled with
`profiles.<name>.sourceSync.mode: off`. The retry loop is bounded by
`profiles.<name>.sourceSync.maxRounds`, defaulting to 8.

The CLI owns generic manifest expansion, local source hashing/listing, source
blob uploads, retry orchestration, and stderr progress. The server remains
authoritative for format-specific source closure and cache/execution
signatures.

## Rationale

Automatic profile-level sync preserves existing command ergonomics and avoids
teaching users a separate preflight step. Server-driven missing-input responses
avoid uploading entire workspaces and avoid duplicating tool-specific closure
logic in the CLI. The bounded retry loop keeps failed or incompatible gateway
behavior from hanging a command indefinitely.

## Contradiction Check

No existing Taidon ADR is made obsolete. This decision extends the existing
remote profile and file-bearing inputset direction with a remote transport
negotiation loop.

