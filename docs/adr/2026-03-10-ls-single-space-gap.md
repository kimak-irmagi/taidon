# ADR: `sqlrs ls` compact single-space gap for states table

Status: Accepted
Date: 2026-03-10

## Decision Record 1: use a 1-character inter-column gap in compact states output

- Timestamp: 2026-03-10T00:00:00Z
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should compact human-readable `sqlrs ls --states` keep the default two-character gap between columns, or reduce it to one character to reclaim width for deep trees and wide state metadata?
- Alternatives:
  - Keep the existing two-character inter-column gap.
  - Reduce the compact human-readable states table to a one-character inter-column gap.
- Decision:
  - Compact human-readable `sqlrs ls --states` uses a one-character gap between columns.
  - This change is specific to the width-sensitive states table and does not, by itself, redefine spacing for the other `ls` tables.
- Rationale: With deep state trees, compact `KIND`, relative `CREATED`, and truncated `PREPARE_ARGS`, the remaining avoidable width cost is the fixed gap between columns. Reducing that gap from two characters to one immediately recovers several columns without reducing information density.

## Contradiction check

No existing ADR was marked obsolete. This decision refines the compact states-table layout and is compatible with the accepted `sqlrs ls` rendering ADRs from 2026-03-10.
