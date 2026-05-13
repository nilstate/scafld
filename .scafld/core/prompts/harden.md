# HARDEN MODE TEMPLATE

This file is the managed harden prompt. Workspace-owned copies may override it
at `.scafld/prompts/harden.md`.

**Status:** ACTIVE
**Mode:** HARDEN
**Output:** Add grounded questions under the latest `## Harden Rounds` entry in the spec; keep `harden_status: "in_progress"` until the operator runs `--mark-passed`.
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Hardening is
not a formatting pass and "Questions: none" is not valid until the audit checks
below are recorded with evidence.

Run these checks before polishing wording:

- Path audit: every named file, directory, package, and generated artifact exists now or is explicitly declared as new.
- Command audit: every validation command is runnable from the declared working directory with the configured toolchain.
- Scope/migration audit: every migration, cutover, compatibility claim, and "no migration needed" statement is backed by repo evidence.
- Acceptance timing audit: every acceptance criterion can be evaluated after the phase that claims it, not before implementation creates its target.
- Rollback/repair audit: every risky phase has a realistic repair or rollback path.
- Design challenge: ask whether the plan is a bandaid, future bloat, compatibility debt, or the wrong abstraction for the product.

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
  - Evidence: The plan fixes the root cause without adding aliases, fallbacks, or compatibility debt.
```

If any check cannot pass, keep the round open and add a grounded question or
rewrite the spec so the check can pass.

Check `Result:` must be `passed` or `not_applicable` before the round can pass.
`not_applicable` still requires evidence.

Work these harden questions after the checks expose the real uncertainty:

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

Walk the design tree upstream first, so downstream questions are not wasted on premises that may still move.

Ask one question at a time. For each question, provide your recommended answer.

If a question can be answered by exploring the codebase, explore the codebase instead of asking. Bring back the verified finding and use it to sharpen the next question.

Record why each question exists with a single `Grounded in:` value:

- `spec_gap:<field>` for a missing, vague, or contradictory spec field
- `code:<file>:<line>` for code you actually verified in this session
- `archive:<task_id>` for a relevant archived spec precedent

Use `Grounded in:` as audit trail, not ceremony. Do not invent citations. Do not cite code you have not read. Do not ask about behavior the spec already settles.

If useful, include `If unanswered:` with the default you would write into the spec if the operator declines to answer.

If the checks pass and you cannot form a genuine grounded question, record:

```markdown
Questions:
- none
```

Do not pad the round.

`max_questions_per_round` from `.scafld/config.yaml` is a cap, not a target.

Record each question in this exact Markdown shape under the latest harden round.
Do not use YAML object keys such as `question:`, `grounded_in:`, `recommended_answer:`, or `resolution:`.

```markdown
Questions:
- Which module owns session cleanup?
  - Grounded in: code:src/auth/session.ts:84
  - Recommended answer: Use the existing cleanupSession owner.
  - If unanswered: Default to the existing cleanup path.
  - Answered with: Use cleanupSession.
```

The operator can end the loop by saying `done` or `stop`. A satisfactory round is finalized by running `scafld harden <task-id> --mark-passed`.
