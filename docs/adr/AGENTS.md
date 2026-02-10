# Architecture desision record agent instructions

When working on the documents in this folder, follow these rules:

1. Each ADR record should contain:
   - The conversation timestamp
   - The github user id involved in the decision (request the value from the user when needed)
   - The agent name/version involved in the decision
   - The question discussed
   - The alternatives considered
   - The chosen solution or decision made
   - The brief rationale of the decision
2. If multiple decisions are made within a single session, each should have a
   separate ADR record, though they may share the same .md file.
3. The ADR files should be named with the date prefix in format YYYY-MM-DD
   (except for the legacy ones created before this rule went into effect)
   and the name reflecting the decision topic.
   Multiple files can be created for a single date in
   case there is are no short enough phrase to
   summarise the topic across all the individual
   decisions.
4. When a new ADR is added, the existing ones should be checked for the
   contradictions. If any discovered, the old ADR(s) should be marked as Obsolete
   with a hyperlink to the new ADR.
