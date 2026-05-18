# HARDEN MODE TEMPLATE

This file is the managed harden prompt. Workspace-owned copies may override it
at `.scafld/prompts/harden.md`.

**Status:** ACTIVE
**Mode:** HARDEN
**Output:** Add grounded checks and issues under the latest `## Harden Rounds` entry in the spec; keep `harden_status: "in_progress"` until the operator runs `--mark-passed`.
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Hardening is
not a formatting pass and "Issues: none" is not valid until the audit checks
below are recorded with evidence. Preserve full detail, but separate approval
blockers from advisories so harden does not loop forever on non-blocking polish.

Run these checks before polishing wording:

- Path audit: every named file, directory, package, and generated artifact exists now or is explicitly declared as new.
- Command audit: every validation command is runnable from the declared working directory with the configured toolchain.
- Scope/migration audit: every migration, cutover, compatibility claim, and "no migration needed" statement is backed by repo evidence.
- Acceptance timing audit: every acceptance criterion can be evaluated after the phase that claims it, not before implementation creates its target.
- Rollback/repair audit: every risky phase has a realistic repair or rollback path.
- Design challenge: challenge the plan's reason for existing. Ask what deeper product, system, or workflow problem it solves; whether the proposed change is a short-sighted bandaid over an endemic issue; whether a smaller, larger, or different abstraction would remove the root cause; and whether it creates future bloat, compatibility debt, or product confusion.

Record checks in this exact Markdown shape under the latest harden round:

```markdown
Checks:
- Path audit
  - Grounded in: code:src/auth/session.ts:84
  - Result: passed
  - Evidence: Existing session owner and target path verified.
- Command audit
  - Grounded in: code:Makefile:12
  - Result: passed
  - Evidence: `make test` is declared from the repository root.
- Scope/migration audit
  - Grounded in: spec_gap:Risks
  - Result: passed
  - Evidence: Migration-free claim removed; risk now names the required cutover.
- Acceptance timing audit
  - Grounded in: spec_gap:Phases
  - Result: passed
  - Evidence: Criteria run after the phase creates the target files.
- Rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: not_applicable
  - Evidence: Docs-only change has no runtime rollback.
- Design challenge
  - Grounded in: spec_gap:Summary
  - Result: passed
  - Evidence: The plan names the underlying problem, fixes the root cause, and avoids aliases, fallbacks, compatibility debt, and future bloat.
```

If any check cannot pass, keep the round open and add a grounded approval-blocking
issue or rewrite the spec so the check can pass.

Check `Result:` must be `passed` or `not_applicable` before the round can pass.
`not_applicable` still requires evidence.

Work these harden challenge points after the checks expose the real uncertainty.
Record the result as an issue:

- Why should this plan exist at all?
- What is the real product/system/workflow problem, not just the requested implementation?
- Is the plan treating a symptom while leaving an endemic problem in place?
- Would a different abstraction remove the root cause more cleanly?
- What is authoritative when two artifacts contain the same fact?
- What are the ownership boundaries?
- What fails halfway, and how is it repaired?
- What invariants must be testable?
- What hidden cutovers are bundled?
- What examples or golden fixtures prove the shape?
- What operational command lets a human recover?
- Can we dogfood this?
- What complexity is being accepted, and why is it worth it?

Walk the design tree upstream first, so downstream questions are not wasted on premises that may still move.

Ask one operator question at a time when manual hardening needs input. Record
each one as a `question` issue with your recommended answer.

If an issue can be resolved by exploring the codebase, explore the codebase instead of asking. Bring back the verified finding and use it to sharpen the next issue.

Record why each question exists with a single `Grounded in:` value:

- `spec_gap:<field>` for a missing, vague, or contradictory spec field
- `code:<file>:<line>` for code you actually verified in this session; cite a
  single anchor line, not a line range
- `archive:<task_id>` for a relevant archived spec precedent

Use `Grounded in:` as audit trail, not ceremony. Do not invent citations. Do not cite code you have not read. Do not ask about behavior the spec already settles.

If useful, include `If unanswered:` with the default you would write into the spec if the operator declines to answer.

Record harden findings in one issue list. Use `blocks approval` only when the
draft is unsafe, incoherent, non-executable, or likely to create a bad
architectural commitment. Use `advisory` when the detail is useful but approval
can proceed.

```markdown
Issues:
- [high/blocks approval] `harden-1` design_challenge - The plan treats a symptom and leaves the root cause in place.
  - Status: open
  - Grounded in: spec_gap:Summary
  - Evidence: The summary names the patch but not the underlying workflow or product problem.
  - Recommendation: Rewrite the summary/objectives to address the root cause, or shrink the plan to the honest local fix.
- [low/advisory] `harden-2` question - The spec could name the operator-facing recovery command.
  - Status: open
  - Grounded in: spec_gap:Rollback
  - Evidence: Rollback is technically enough, but the next human would benefit from a named recovery command.
  - Recommendation: Add the recovery command if it is already known.
  - Question: What command should a human run if the cutover fails halfway?
  - Recommended answer: Use the existing repair command documented by the affected package.
  - If unanswered: Leave rollback as-is; do not block approval.
```

If the checks pass and you cannot form a genuine grounded issue, record:

```markdown
Issues:
- none
```

Do not pad the round. `max_issues_per_round` from `.scafld/config.yaml` is a
cap, not a target.

Approval-blocking issues must be fixed, marked `fixed`, marked
`accepted_risk`, or superseded before the round can pass. Advisory issues may
remain open. Do not use YAML object keys such as `question:`, `grounded_in:`,
`recommended_answer:`, or `resolution:`.

The operator can end the loop by saying `done` or `stop`. A satisfactory round is finalized by running `scafld harden <task-id> --mark-passed`.
