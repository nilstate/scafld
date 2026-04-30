---
spec_version: '2.0'
task_id: review-clean-section-ergonomics
created: '2026-04-25T09:44:33Z'
updated: '2026-04-26T11:12:23Z'
status: completed
harden_status: passed
size: small
risk_level: low
---

# Make clean review sections easier to write correctly

## Current State

Status: completed
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

Fix the review-gate ergonomics gap where semantically clean adversarial sections can still fail parsing if the reviewer writes a natural variant like "No additional issues found — checked ..." instead of the one exact accepted phrase. The clean model is: the parser accepts a small family of explicit clean-section notes, and the docs/tests teach the accepted forms directly.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/reviewing.py` (task-selected) - The no-issues parser is the source of the current phrase brittleness.
- `tests/review_gate_smoke.sh` (task-selected) - Review gate smoke should prove the natural clean-section variant now passes.
- `docs/review.md` (task-selected) - The documented accepted forms should match the parser.

Invariants:
- `clean_review_sections_must_be_easy_to_write_correctly`
- `review_gate_parser_must_stay_strict_about_evidence`
- `packaged_cli_is_the_truth_surface`

Related docs:
- none

## Objectives

- Accept a small family of explicit no-issues review notes without weakening finding validation.
- Make the review docs teach the accepted clean-section forms directly.
- Prove the behavior through the real review gate smoke.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- review parsing: Adversarial review sections should accept natural clean-section notes that still state what was checked.
- review docs: The operator-facing guidance should match the parser exactly.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` A review round using "No additional issues found — checked ..." passes the gate without weakening finding validation.
- [ ] `dod2` The review docs describe the accepted clean-section note forms.

Validation:
- [ ] `v1` compile - Compile the Python sources after the review parser update.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke proves the clean-section variant passes.
  - Command: `bash tests/review_gate_smoke.sh clean-section-variants`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Widen clean-section parsing and teach it

Goal: Accept the natural clean-section variant we actually hit, document the accepted forms, and prove it through the raw review gate smoke.

Status: completed
Dependencies: none

Changes:
- `scafld/reviewing.py` (all) - Expand the accepted clean no-issues note forms while keeping finding validation strict.
- `tests/review_gate_smoke.sh` (all) - Add a targeted review-gate smoke case that uses the accepted variant phrase and proves the gate passes.
- `docs/review.md` (all) - Document the accepted clean-section note forms so manual reviewers do not have to guess.

Acceptance:
- [x] `ac1_1` integration - Review gate smoke proves the natural clean-section variant passes.
  - Command: `bash tests/review_gate_smoke.sh clean-section-variants`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: per_phase

Commands:
- none

## Review

Status: not_started
Verdict: pass
Timestamp: '2026-04-26T11:11:47Z'
Review rounds: 1
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: 0
Non-blocking count: none

Findings:
- none

Passes:
- `id`: spec_compliance
- `id`: scope_drift
- `id`: regression_hunt
- `id`: convention_check
- `id`: dark_patterns

## Self Eval

Status: not_started
Completeness: none
Architecture fidelity: none
Spec alignment: none
Validation depth: none
Total: none
Second pass performed: none

Notes:
none

Improvements:
- none

## Deviations

- none

## Metadata

Estimated effort hours: none
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- none

## Origin

Source:
- none

Repo:
- none

Git:
- none

Sync:
- none

Supersession:
- none

## Harden Rounds

- none

## Planning Log

- 2026-04-25T09:44:33Z - user - Spec created via scafld plan
- 2026-04-25T09:45:56Z - assistant - Replaced the placeholder draft with a parser-and-docs fix grounded in the real review phrase mismatch hit during dogfooding.
- 2026-04-26T11:11:20Z - cli - Spec approved
- 2026-04-26T11:11:21Z - cli - Execution started
- 2026-04-26T11:12:23Z - cli - Spec completed
