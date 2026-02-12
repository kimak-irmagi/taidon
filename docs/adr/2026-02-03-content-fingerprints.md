# ADR 2026-02-03: Content-based fingerprints for psql and liquibase

Status: Accepted
Date: 2026-02-03

## Decision Record

- Timestamp: 2026-02-03T10:30:00+00:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should state fingerprints be derived from command arguments or from content for psql and liquibase?
- Alternatives:
  - Keep argument-based fingerprints (paths + flags + input hashes).
  - Use content-based fingerprints:
    - psql: normalized SQL content with all include directives expanded.
    - liquibase: per-changeset hash from Liquibase checksum, fallback to updateSQL content.
- Decision:
  - Use **content-based** fingerprints for `prepare:psql` and `prepare:lb`.
- Rationale:
  - Argument or include-path differences should not create new states if the resolved
    content is identical.
  - Liquibase already derives stable changeset checksums; when unavailable, updateSQL
    provides a deterministic content source.
