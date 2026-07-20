# 2026-07-20 Absolute Client Path Binding for Remote Sources

- Conversation timestamp: 2026-07-20T11:52:53.0950999+07:00
- GitHub user id involved in the decision: @evilguest (41718235)
- Agent name/version involved in the decision: Codex / GPT-5
- Status: Accepted

## Question Discussed

How should file-bearing prepare and cache-explain arguments identify local
workspace files when a client and a remote Taidon deployment do not share a
filesystem or operating-system path syntax?

## Alternatives Considered

1. Preserve absolute client host paths in `psql_args`, `liquibase_args`, and
   `work_dir`, and attempt to open them directly in the remote runner.
2. Convert every file-bearing request to inline stdin or an eagerly uploaded
   workspace archive, bypassing the accepted manifest negotiation loop.
3. Preserve absolute client paths as logical invocation coordinates, add the
   client workspace root and effective working directory to the source
   manifest context, and project the admitted request to runner-local paths
   only after authoritative source resolution.

## Chosen Solution Or Decision Made

Choose alternative 3.

The CLI continues to bind file-bearing arguments to absolute native client
paths before it sends either local or remote requests. For remote source sync,
`source_manifest.workspace_ref` also carries the absolute logical workspace
root and effective command working directory. Those values use the same client
path flavor as the request arguments and are coordinates for source resolution;
they are never server filesystem locations.

Manifest keys and `source_inputs_missing` paths remain workspace-relative and
slash-separated. During admission, the authoritative resolver rebases absolute
client paths against the supplied logical root, preserves the distinct `psql`
`\i` and `\ir` bases, and produces a private workspace-relative execution
projection. Only that projection is materialized as runner-local absolute paths.

## Brief Rationale

Absolute paths preserve the host-side binding selected for raw arguments,
aliases, refs, `psql` includes, and Liquibase search paths. Rewriting them in
the CLI would discard information needed to diagnose the exact missing import.
At the same time, a Windows path cannot be opened by a Linux runner. An explicit
logical root/work-dir binding preserves the accepted client contract while
letting admission translate it deterministically into the portable manifest
namespace and then into the runner filesystem.

## Contradiction Check

This decision refines, but does not obsolete,
`2026-07-06-remote-source-input-sync-cli.md`,
`2026-03-22-shared-inputset-layer.md`, or
`2026-03-23-psql-include-base-semantics.md`. It preserves the existing absolute
prepare-path contract and makes the previously implicit client-to-virtual
workspace binding explicit.
