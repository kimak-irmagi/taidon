# Taidon agent instructions

When working on this project, follow these rules:

0. When modifying or creating an artifact within a workspace folder, check the
   agents.md file contents at that folders and at all the parent folders toward
   the workspace root. When rules in the files conflict, the parent rules take
   precedence over the child rules.
1. When working on a feature, design it first:
   - Design the CLI syntax, if it is involved. Ask questions, if the goal is
     unclear. Get an approval from the user before proceeding.
     Store the result in an apropriate document within /docs/user-guides. Create
     a new document if no good fit exists. Get an approval from the user before
     proceeding.
   - Once approval is granted, review all the other documents in /docs/architecture,
     and README.md files across the project to locate any inconsistencies with
     the new design, and fix those inconsistencies.
   - Design an OpenAPI spec, if API is involved. Ask questions, if the goal is
     unclear. Store it in the /docs/api-guides/sqlrs-engine.openapi.yaml. Get an
     approval from the user before proceeding.
   - Once approval is granted, review all the other documents in /docs/architecture,
     and README.md files across the project to locate any inconsistencies with
     the new design, and fix those inconsistencies.
   - Design the component interaction flow. Store it within an appropriate document
     in /docs/architecture. Create a new document if no good fit exists.
     Get an approval from the user before proceeding.
   - Design internal component structure for each deployment unit (CLI, engine,
     services): define packages/modules, responsibilities, key types/interfaces,
     and data ownership (in-memory vs persistent). Store it in an appropriate
     document in /docs/architecture (create a new one if needed). Get an approval
     from the user before proceeding.
     Once approval is granted, review all the other documents in /docs/architecture,
     and README.md files across the project to locate any inconsistencies with
     the new design, and fix those inconsistencies.
   - Design the DB schema changes, if any are involved. Get an approval from the
     user before proceeding.
   - At any stage, whenever there are multiple ways to design a particular item,
     store the information on the decision taken in an appropriate document at
     the docs/adr folder. Create a new document if no good fit exits. This covers
     both decisions proposed by user as well as the decisions suggested by an agent
     and approved by the user.
     If docs/adr does not exist yet, create it before adding the ADR.
     Each ADR record should contain:
     - The conversation timestamp
     - The github user id involved in the decision
     - The agent name/version involved in the decision
     - The question discussed
     - The alternatives considered
     - The chosen solution or decision made
     - The brief rationale of the decision
2. Once the design is approved and the documents are updated, design the tests
   for the feature.
   - Show the list of the new tests to the user and get approval.
   - Once the list is approved, review the existing tests searching for the
     contradictions. If any contradictions are found, ask the user what to do:
     fix the new tests or the old ones.
3. Once the tests are approved, start bulding the tests.
4. Once the tests are ready, write the code to pass those tests.
   Do not skip tests or alter them unless explicitly requested by user.
5. Use in-line and doc-comments where appropriate to describe each
   function/class/type/member purpose and behavior (requrements on the input
   parameters, return values, invariants maintained, etc). Make sure to reference
   the architecture documents from which these requirements are derived.  
6. Once the code is written, run the tests, fix any issues, and measure the code
   coverage. Target value is 100% coverage, with the acceptable minimum is 95%.
   The coverage deficiencies should be addressed as follows:
   - Obtain the detailed per-line coverage report
   - Start with the files with the highest count of the uncovered lines.
     Move on to the next file with the highest count of the uncovered lines if
     the overall target is not met, and so on.
   - Trace the uncovered lines back to the code requirements:
     - if there are some additional requirements implied by the implementation,
       but not explicitly documented - plan documenting those requirements
       and adding the tests that test these requirements (not the undocumented
       implementation details!)
     - if there are no actual requirements, consider the uncovered lines to be
       a dead code and plan it for removal 
   - The resulting plan for the test coverage increase should be approved by the
     user
   - once approved, proceed with the plan.
   - once changes are applied, re-measure the coverage. 
   - if the coverage is still below target, ask the user for a permission to
     perform one more iteration
   
