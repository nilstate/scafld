package prompts

// Harden is the built-in default prompt used to drive spec hardening rounds.
const Harden = `# HARDEN MODE TEMPLATE

This is the built-in default harden prompt. Workspace-owned copies may override
it at .scafld/prompts/harden.md.

**Status:** ACTIVE
**Mode:** HARDEN
**Output:** Add grounded checks and issues under the latest ## Harden Rounds entry in the spec; keep harden_status in_progress until the operator runs --mark-passed.
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Preserve
full detail, but separate approval blockers from advisories so harden does not
loop forever on non-blocking polish.

Run path, command, scope/migration, acceptance timing, rollback/repair, and
design challenge checks before polishing wording.

Work these harden questions after the checks expose real uncertainty:

- What is the real product goal, not just the requested implementation?
- What is authoritative when two artifacts contain the same fact?
- What are the ownership boundaries?
- What fails halfway, and how is it repaired?
- What invariants must be testable?
- What hidden cutovers are bundled?
- What examples or golden fixtures prove the shape?
- What operational command lets a human recover?
- Can we dogfood this?
- What complexity is being accepted, and why is it worth it?

Ask one question at a time when manual hardening needs operator input. For each
question, provide your recommended answer.
If a question can be answered by exploring the codebase, explore the codebase instead of asking.
Every question must carry one Grounded in value: spec_gap:<field>, code:<file>:<line>, or archive:<task_id>.

Use Grounded in as audit trail, not ceremony. Do not invent citations. Do not cite code you have not read. Do not ask about behavior the spec already settles.

If useful, include If unanswered with the default you would write into the spec if the operator declines to answer.

Record findings in one Issues list. Use blocks approval only when the draft is
unsafe, incoherent, non-executable, or likely to create a bad architectural
commitment. Use advisory when the detail is useful but approval can proceed.

If you cannot form a genuine grounded issue, record Issues: none. Do not pad the
round.

Record each issue in this exact Markdown shape under the latest harden round.
Do not use YAML object keys such as question:, grounded_in:, recommended_answer:, or resolution.

` + "```markdown" + `
Issues:
- [high/blocks approval] ` + "`" + `harden-1` + "`" + ` question - Which module owns session cleanup?
  - Status: open
  - Grounded in: code:src/auth/session.ts:84
  - Evidence: Existing cleanup owner verified in code.
  - Recommendation: Use the existing cleanupSession owner.
  - Question: Which module owns session cleanup?
  - Recommended answer: Use the existing cleanupSession owner.
  - If unanswered: Default to the existing cleanup path.
` + "```" + `

The operator can end the loop by saying done or stop. A satisfactory round is finalized by running scafld harden <task-id> --mark-passed.
`
