# 2026-07-20 Dedicated Source Transfer Timeout

- Conversation timestamp: 2026-07-20T20:22:03.0377476+07:00
- GitHub user id involved in the decision: @evilguest (41718235)
- Agent name/version involved in the decision: Codex / GPT-5
- Status: Accepted

## Question Discussed

How should the CLI time out remote source blob uploads that can be much larger
than ordinary control-plane requests?

## Alternatives Considered

1. Keep using the common 30-second HTTP client timeout.
2. Increase the timeout for every CLI HTTP request.
3. Add a new user-facing CLI flag or profile setting for upload timeout.
4. Keep the configured control timeout and use a dedicated 15-minute deadline
   for source blob transfer operations.

## Chosen Solution Or Decision Made

Choose alternative 4. Source blob uploads use a separate authenticated HTTP
transfer client with a 15-minute default deadline. Prepare create, status, and
other control requests retain their existing timeout semantics. No CLI syntax
or configuration key is added.

## Brief Rationale

The maintained `flights-multi-step` example contains a 106,128,119-byte source
file. A 30-second whole-request deadline is too short on ordinary uplinks, while
raising every request timeout would hide control-plane failures. A separate
deadline matches the distinct streaming workload without changing user-facing
command syntax.

## Contradiction Check

This decision refines but does not obsolete
`2026-07-06-remote-source-input-sync-cli.md` and
`2026-07-20-portable-remote-source-path-binding.md`.

