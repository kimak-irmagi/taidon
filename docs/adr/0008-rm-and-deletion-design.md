# ADR 0008: rm command and deletion design

Status: Accepted
Date: 2026-01-16

## Decision Record 1: rm CLI semantics and output

- Timestamp: 2026-01-16T13:10:21+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: What are the semantics and safety rules for `sqlrs rm`?
- Alternatives:
  - Require interactive confirmation for destructive deletes.
  - Allow partial deletion when some descendants are blocked.
  - Use a job-based async delete with polling.
- Decision: Use `sqlrs rm <id_prefix>` with prefix resolution (hex, min 8, case-insensitive). If the prefix is ambiguous across instances/states, error. If nothing matches, warn and exit 0. Deletion is synchronous. `--recurse` is required for states with descendants. `--force` ignores active connections but does not imply `--recurse`. If any descendant is blocked (active connections without `--force`), no deletions occur. `--dry-run` returns the same tree output without changes. Human output shows a tree with `deleted`/`would delete` and `blocked` reasons; JSON output includes `dry_run` plus node-level `blocked` codes.
- Rationale: This matches user expectations for safety and idempotency, avoids surprising partial results, and keeps the CLI simple while remaining scriptable.

## Decision Record 2: delete API shape, ancestry, and connection tracking

- Timestamp: 2026-01-16T13:10:21+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine expose deletion, ancestry, and connection tracking?
- Alternatives:
  - Use `POST /v1/delete-jobs` with async job tracking.
  - Return 204 without a response body on delete.
  - Persist live connection counts in SQLite.
- Decision: Provide idempotent `DELETE /v1/instances/{id}` and `DELETE /v1/states/{id}` endpoints with `recurse`, `force`, and `dry_run` query parameters and a structured `DeleteResult` tree response. States store `parent_state_id` to model ancestry for recursive deletion. Live connection counts are tracked in memory and exposed as `connections` for instance nodes in delete responses.
- Rationale: DELETE aligns with REST semantics, the deletion tree makes safety decisions explicit, ancestry is required for recursive deletes, and in-memory connection tracking avoids persistence of volatile data.

## Decision Record 3: dry-run HTTP semantics

- Timestamp: 2026-01-16T13:38:52+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the engine indicate "would delete" vs "blocked" for dry-run without misusing HTTP status codes?
- Alternatives:
  - Use 200 for success and 409 for blocked even in dry-run.
  - Always return 200 for dry-run and expose the result in the response body.
  - Add a separate delete-check endpoint.
- Decision: For `dry_run=true`, the engine always returns 200 and includes an `outcome` field (`would_delete` or `blocked`) in the response body. For non-dry-run deletes, 409 remains the signal for blocked deletions.
- Rationale: This keeps dry-run idempotent and non-erroring at the HTTP level while still exposing clear machine-readable results.
