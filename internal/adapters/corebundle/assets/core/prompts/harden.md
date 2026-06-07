# HARDEN MODE TEMPLATE

This file is the managed harden prompt. Workspace-owned copies may override it
at `.scafld/prompts/harden.md`.

**Status:** ACTIVE
**Mode:** HARDEN
**Output:** Fill the generated observation rows under the latest `## Harden Rounds` entry in the spec; keep `harden_status: "in_progress"` until the operator runs `--mark-passed`.
**Do NOT:** Modify code outside the spec file while hardening.

---

Interrogate the draft spec until it is executable without invention. Hardening is
not a formatting pass, and a clean-looking round is not valid until every row has
a grounded, filesystem-verifiable anchor.

Cover these six dimensions before polishing wording:

- `path`: every named file, directory, package, and generated artifact exists now or is explicitly declared as new.
- `command`: every validation command is runnable from the declared working directory with the configured toolchain.
- `scope`: every migration, cutover, compatibility claim, and "no migration needed" statement is backed by repo evidence.
- `timing`: every acceptance criterion can be evaluated after the phase that claims it, not before implementation creates its target.
- `rollback`: every risky phase has a realistic repair or rollback path.
- `design`: challenge why the plan exists, what root problem it solves, whether it is a short-sighted bandaid, and whether it creates future bloat, compatibility debt, or product confusion.

Manual rounds already contain the six required observation rows. Fill the
existing `Result` and `Anchor` fields; add `Note`, `Default`, or `Status` only
when needed. Do not delete or rename dimensions. If rows are missing because the
spec was edited by hand, restore this exact shape under the latest harden round:

```markdown
Observations:
- path
  - Result: clean
  - Anchor: code:src/auth/session.ts:84
  - Note: Existing session owner and target path verified.
- command
  - Result: clean
  - Anchor: code:Makefile:12
  - Note: `make test` is declared from the repository root.
- scope
  - Result: advisory
  - Anchor: spec_gap:Risks
  - Note: Migration-free claim is plausible but could name the affected deployment path.
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Criteria run after the phase creates the target files.
- rollback
  - Result: n/a
  - Anchor: spec_gap:Rollback
  - Note: Docs-only change has no runtime rollback.
- design
  - Result: blocks
  - Anchor: spec_gap:Summary
  - Note: The summary names the patch but not the underlying workflow or product problem.
  - Default: Rewrite the summary/objectives to address the root cause, or shrink the plan to the honest local fix.
  - Status: open
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

Use `Default` only when the note is effectively a question and a reasonable
default answer exists. Ask one operator question at a time when manual hardening
needs input, then record the question in `Note` and your proposed default in
`Default`.

If a blocking observation is resolved, set `Status: fixed`, `accepted_risk`, or
`superseded`. Open blocking observations keep harden not-ready. Advisory
observations may remain open and do not block approval.

Provider hardening must call `submit_harden` exactly once with the final
HardenDossier. Do not write a verdict; scafld derives it from dimension coverage
and unresolved `blocks` observations. Do not emit final prose or raw JSON text.

Do not pad the round. `max_issues_per_round` from `.scafld/config.yaml` remains
a cap on useful findings, not a target. The operator can end the loop by saying
`done` or `stop`. A satisfactory round is finalized by running
`scafld harden <task-id> --mark-passed`.
