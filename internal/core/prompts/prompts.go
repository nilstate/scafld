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

Optimize for finding as many real spec issues as the round budget allows. Spend
the budget on grounded blockers and useful advisories; do not pad the round with
speculative or weak observations just to increase the count.

Cover these six dimensions in this order:

- design
- scope
- path
- command
- timing
- rollback

The design row is the right-to-exist and architecture challenge. Ask why this
plan should exist, what root problem it solves, whether it is the smallest
coherent shape, and whether shared behavior belongs in a common core/app
contract with thin adapters for API, MCP, CLI, provider, or docs surfaces.

Each observation needs:

- Dimension: one of design, scope, path, command, timing, rollback.
- Result: clean, advisory, blocks, or n/a.
- Anchor: spec_gap:<field>, code:<file>:<line>, or archive:<task_id>.
- Note: required for advisory and blocks; useful for n/a when the reason is not obvious.
- Default: optional answer for question-shaped observations.
- Status: open, fixed, accepted_risk, or superseded for blocking observations.

Use anchors as audit trail, not ceremony. Do not invent citations. Do not cite
code you have not read. Do not ask about behavior the spec already settles.

Work these harden questions before path and command bookkeeping, then record the
evidence in the observation ledger:

- What is the real product goal, not just the requested implementation?
- What is authoritative when two artifacts contain the same fact?
- What shared core/app contract owns the behavior?
- Are API, MCP, CLI, provider, and docs surfaces light adapters over that shared behavior, or are they drifting into separate implementations?
- What adapter-specific mapping tests are needed, and what shared behavior test proves the rule once?
- What hidden cutovers are bundled?
- What invariants must be testable?
- What examples or golden fixtures prove the shape?
- What fails halfway, and how is it repaired?
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

Treat max_issues_per_round as a budget for real findings: use as much of it as
grounded issues justify, and use none of it for filler.

Record observations in this exact Markdown shape under the latest harden round:

` + "```markdown" + `
Observations:
- design
  - Result: blocks
  - Anchor: spec_gap:Summary
  - Note: The plan treats a symptom and does not name the underlying problem or shared ownership boundary.
  - Default: Rewrite the summary to address the root cause and name the shared owner, or shrink the plan.
  - Status: open
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Shared behavior and adapter-specific surface area are named.
- path
  - Result: clean
  - Anchor: code:src/auth/session.ts:84
  - Note: Existing cleanup owner verified in code.
` + "```" + `

The operator can end the loop by saying done or stop. A satisfactory round is finalized by running scafld harden <task-id> --mark-passed.
`
