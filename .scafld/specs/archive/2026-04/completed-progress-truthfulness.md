---
spec_version: '2.0'
task_id: completed-progress-truthfulness
created: '2026-04-25T09:35:43Z'
updated: '2026-04-25T09:44:03Z'
status: completed
harden_status: passed
size: small
risk_level: medium
---

# Make completed task progress truthful

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

Fix the progress-truth gap where archived completed specs can still render as `[0/x]` in `scafld list` because their per-phase status fields were never finalized. The clean model is: completing a spec stamps truthful terminal phase and definition-of-done state into the archived document, and read surfaces count older completed archives truthfully even before they are rewritten.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/commands/review.py` (task-selected) - Completion is the right lifecycle boundary to stamp terminal truth into the archived spec.
- `scafld/spec_parsing.py` (task-selected) - Phase counting currently trusts stale archived phase statuses and drives `scafld list`.
- `tests/completed_progress_truth_smoke.sh` (all) - A raw CLI smoke should prove both legacy archive projection and new completion behavior.

Invariants:
- `completed_specs_must_not_render_partial_progress`
- `archived_specs_should_store_terminal_truth`
- `packaged_cli_is_the_truth_surface`

Related docs:
- none

## Objectives

- Make `scafld list` show truthful completed progress for both legacy and newly completed specs.
- Stamp completed archives with terminal phase and DoD truth at completion time.
- Prove the behavior through the raw packaged CLI instead of helper internals.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- completion lifecycle: Completing a spec should normalize terminal truth into the archived file.
- phase counting: Read surfaces should not trust stale archived phase statuses for completed specs.
- archive UX: "scafld list" should never show "completed ... [0/x]".

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Newly completed specs archive with all phases marked completed and all DoD entries marked done.
- [ ] `dod2` Legacy completed archives with stale phase statuses still render truthful completed progress in "scafld list".

Validation:
- [ ] `v1` compile - Compile the Python sources after the completion-truth fix.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Raw CLI smoke proves both legacy archive projection and new completion stamping are truthful.
  - Command: `bash tests/completed_progress_truth_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Normalize completed archive truth

Goal: Make completed archive state truthful both when written today and when projected from older archived specs.

Status: completed
Dependencies: none

Changes:
- `scafld/commands/review.py` (all) - Normalize completed specs before archiving so phases are terminal and DoD entries are marked done.
- `scafld/spec_parsing.py` (all) - Count completed archived phases truthfully even when older archive files still carry stale pending phase statuses.
- `tests/completed_progress_truth_smoke.sh` (all) - Add a raw CLI smoke that proves legacy completed archives render truthfully and new completion stamps truthful terminal state into the archived spec.

Acceptance:
- [x] `ac1_1` integration - Raw CLI smoke proves completed progress is truthful for both legacy and newly completed specs.
  - Command: `bash tests/completed_progress_truth_smoke.sh`
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
Timestamp: '2026-04-25T09:43:39Z'
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

- 2026-04-25T09:35:43Z - user - Spec created via scafld plan
- 2026-04-25T09:39:05Z - assistant - Replaced the placeholder draft with a concrete write-and-read truth fix for completed archive progress.
- 2026-04-25T09:38:09Z - cli - Spec approved
- 2026-04-25T09:38:43Z - cli - Execution started
- 2026-04-25T09:44:03Z - cli - Spec completed
