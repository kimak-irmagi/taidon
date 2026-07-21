# Source Sync Progress Presentation Decisions

Status: Accepted.

Conversation timestamp: `2026-07-21T17:55:22.0978361+07:00`

GitHub user id: `evilguest`

Agent: `Codex / GPT-5`

## Decision 1: Progress Ownership

### Question

How should remote source synchronization expose detailed progress consistently
in verbose, interactive, and redirected CLI execution?

### Alternatives considered

1. Keep aggregate strings written directly by `internal/remotesource`.
2. Pass presentation callbacks into `internal/remotesource` and let it choose
   terminal formatting.
3. Emit typed semantic events from `internal/remotesource` and let
   `internal/app` select the renderer.

### Decision

Choose alternative 3. The source-sync domain emits typed events. The app layer
renders every event as a verbose stderr line, uses a delayed current-operation
spinner in normal TTY mode, and stays silent for routine progress in normal
non-TTY mode. Stdout is unchanged.

### Rationale

This matches the CLI's shared progress principle, keeps synchronization
testable without terminal behavior, and gives all source-bearing commands one
presentation contract.

## Decision 2: Upload Byte Event Granularity

### Question

Should upload progress emit an event for every stream read?

### Alternatives considered

1. Emit on every `Read` call.
2. Emit only upload start and completion.
3. Emit start, bounded monotonic byte checkpoints, and completion from a
   counting reader.

### Decision

Choose alternative 3. The event stream reports bytes actually consumed by the
upload and coalesces intermediate updates at an implementation-owned bounded
checkpoint. Small uploads still receive completion with their exact byte count.

### Rationale

Actual stream consumption is truthful, while bounded checkpoints avoid making
event volume depend on transport buffer fragmentation. Verbose mode still
renders every semantic progress event it receives.

## Contradiction Check

No existing ADR is obsolete. This refines the CLI ownership of stderr progress
accepted in `2026-07-06-remote-source-input-sync-cli.md` and preserves the
absolute logical path contract from
`2026-07-20-portable-remote-source-path-binding.md`; only safe event labels use
workspace-relative paths.
