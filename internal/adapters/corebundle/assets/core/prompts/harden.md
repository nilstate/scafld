# HARDEN MODE TEMPLATE

This file is the managed harden prompt. Workspace-owned copies may override it
at `.scafld/prompts/harden.md`.

**Status:** ACTIVE
**Mode:** HARDEN
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Hardening is
not a formatting pass, and a clean-looking round is not valid until every row has
a grounded, filesystem-verifiable anchor.

Optimize for finding as many real spec issues as the round budget allows. Spend
the budget on grounded blockers and useful advisories; do not pad the round with
speculative or weak observations just to increase the count.

First answer the shape gate:

Treat the draft as a hypothesis, not a conclusion. The Source Spec Markdown is
the canonical input under review, but it is not evidence that the proposed
shape, owner, or amount of work is right. Re-derive the root problem from the
spec and repo evidence before accepting the plan.

Before choosing `keep`, test the stronger alternatives:

- `reject` / no-op: the task should not exist, or existing behavior already solves it.
- `shrink`: the goal is valid, but a smaller change solves the same root problem.
- `reframe`: the goal is valid, but ownership, architecture, or product framing is wrong.
- reuse or move ownership: existing shared behavior should be used or extended instead of adding a parallel surface.

If a materially better shape solves the same root problem, report shrink or
reframe as the shape decision; do not bury it as advisory feedback.

- `keep`: the draft should proceed as written.
- `shrink`: the goal is valid, but the draft is doing too much.
- `reframe`: the goal is valid, but the architecture or owner is wrong.
- `reject`: the draft should not be approved.

A passing harden round needs `Shape decision: keep`, a true shape, minimal plan,
shared owner, adapter-boundary judgment, and `Required spec edits: none`. Any
other decision or any required spec edit keeps harden in `needs_revision` until
the Markdown spec changes.

`keep` is an earned decision, not the default. A keep decision must state why
`reject`/no-op, `shrink`, `reframe`, and reuse of existing behavior were rejected
for this task. A clean result must say what was checked, what evidence was
inspected, and what would have failed the check. Do not praise the approach as
evidence.

Cover these six dimensions in this order before polishing wording:

- `design`: challenge the plan's right-to-exist and architecture: why it exists, what root problem it solves, whether shared behavior belongs in a shared core/app contract, whether API, MCP, CLI, provider, and docs surfaces stay light adapters, and whether it creates future bloat, compatibility debt, or product confusion.
- `scope`: every migration, cutover, compatibility claim, shared behavior boundary, adapter-specific responsibility, and "no migration needed" statement is backed by repo evidence.
- `path`: every named file, directory, package, and generated artifact exists now or is explicitly declared as new.
- `command`: every validation command is runnable from the declared working directory with the configured toolchain.
- `timing`: every acceptance criterion can be evaluated after the phase that claims it, not before implementation creates its target.
- `rollback`: every risky phase has a realistic repair or rollback path.

Treat scaffold boilerplate as a spec gap. Phase titles or objectives such as
"Implementation", "Complete the requested change", or "Implement the requested
behavior" are not executable contracts unless the surrounding spec has already
replaced them with concrete behavior, files, and acceptance evidence.

If harden evidence is recorded in Markdown, use this shape:

```markdown
Shape decision: reframe
True shape: Move shared behavior into one core contract with thin CLI/API/MCP/provider adapters.
Minimal plan: Specify the shared contract and one adapter mapping test before implementation.
Shared owner: internal/core/example
Adapter boundaries: CLI renders command output; provider consumes the shared context; docs describe the same contract
Required spec edits: Rewrite Scope to name the shared owner; Split adapter-only work out of Phase 1

Observations:
- design
  - Result: blocks
  - Anchor: spec_gap:Summary
  - Note: The summary names the patch but not the underlying workflow, shared owner, or adapter boundary.
  - Question: What root workflow and shared owner is this task actually about?
  - Recommended answer: Rewrite the summary/objectives to address the root cause and name the shared behavior owner, or shrink the plan to the honest local fix.
  - If unanswered: Treat this as a required spec edit before approval.
  - Status: open
- scope
  - Result: advisory
  - Anchor: spec_gap:Risks
  - Note: Migration-free claim is plausible but could name the affected deployment path and adapter surfaces.
- path
  - Result: clean
  - Anchor: code:src/auth/session.ts:84
  - Note: Existing session owner and target path verified; this would fail if the planned import crossed package visibility or layer boundaries.
- command
  - Result: clean
  - Anchor: code:Makefile:12
  - Note: `make test` is declared from the repository root.
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Criteria run after the phase creates the target files.
- rollback
  - Result: n/a
  - Anchor: spec_gap:Rollback
  - Note: Docs-only change has no runtime rollback.
```

Use these result values:

- `clean`: the dimension was checked and no issue remains.
- `advisory`: useful non-blocking feedback remains; include `Note`.
- `blocks`: approval is unsafe, incoherent, non-executable, or architecturally harmful until resolved; include `Note` and `Status`.
- `n/a`: the dimension was checked and does not apply; include enough note to explain why if the reason is not obvious.

Use these anchors:

- `spec_gap:<field>` for a missing, vague, or contradictory spec field.
- `code:<file>:<line>` for code you actually verified in this session; cite a single anchor line, not a range.
- `archive:<task_id>` for a relevant archived spec precedent.

Use `Anchor` as audit trail, not ceremony. Do not invent citations. Do not cite
code you have not read. Do not ask about behavior the spec already settles.

For blocking observations, prefer the fuller triplet: `Question`, `Recommended
answer`, and `If unanswered`. The `If unanswered` line is the default action
that prevents another round trip when the operator does not answer.

If a blocking observation is resolved, set `Status: fixed`, `accepted_risk`, or
`superseded`. Open blocking observations keep harden not-ready. Advisory
observations may remain open and do not block approval.

If the task has no API, MCP, CLI, provider, docs, or other adapter surface,
leave `Adapter boundaries` empty. Do not write `none` or invent a boundary.

Do not pad the round. `max_issues_per_round` from `.scafld/config.yaml` is a
budget for real findings: use as much of it as grounded issues justify, and use
none of it for filler.
