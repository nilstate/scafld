package prompts

// Harden is the built-in default prompt used to drive spec hardening rounds.
const Harden = `# HARDEN MODE TEMPLATE

This is the built-in default harden prompt. Workspace-owned copies may override
it at .scafld/prompts/harden.md.

**Status:** ACTIVE
**Mode:** HARDEN
**Output:** Fill grounded observations under the latest ## Harden Rounds entry in the spec; keep harden_status in_progress until the operator runs --mark-passed.
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Preserve
full detail, but record it as one observation ledger rather than separate checks
and issues. scafld derives the verdict; do not write one manually.

Cover these six dimensions:

- path
- command
- scope
- timing
- rollback
- design

Each observation needs:

- Dimension: one of path, command, scope, timing, rollback, design.
- Result: clean, advisory, blocks, or n/a.
- Anchor: spec_gap:<field>, code:<file>:<line>, or archive:<task_id>.
- Note: required for advisory and blocks; useful for n/a when the reason is not obvious.
- Default: optional answer for question-shaped observations.
- Status: open, fixed, accepted_risk, or superseded for blocking observations.

Use anchors as audit trail, not ceremony. Do not invent citations. Do not cite
code you have not read. Do not ask about behavior the spec already settles.

Work these harden questions after the observations expose real uncertainty:

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

Ask one question at a time when manual hardening needs operator input. If the
question can be answered by exploring the codebase, explore the codebase instead.
Record the question in Note and your proposed default in Default.

Use blocks only when the draft is unsafe, incoherent, non-executable, or likely
to create a bad architectural commitment. Advisory observations may remain open
and do not block approval. Open blocking observations keep harden not-ready until
they are fixed, accepted_risk, or superseded.

Record observations in this exact Markdown shape under the latest harden round:

` + "```markdown" + `
Observations:
- path
  - Result: clean
  - Anchor: code:src/auth/session.ts:84
  - Note: Existing cleanup owner verified in code.
- design
  - Result: blocks
  - Anchor: spec_gap:Summary
  - Note: The plan treats a symptom and does not name the underlying problem.
  - Default: Rewrite the summary to address the root cause or shrink the plan.
  - Status: open
` + "```" + `

The operator can end the loop by saying done or stop. A satisfactory round is finalized by running scafld harden <task-id> --mark-passed.
`
