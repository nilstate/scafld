---
spec_version: '2.0'
task_id: scafld-self-write-excluded-from-review-anchor
created: '2026-04-27T15:28:17Z'
updated: '2026-04-27T15:32:49Z'
status: completed
harden_status: not_run
size: small
risk_level: medium
---

# Exclude scafld control-plane writes from review-anchor diff

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

`scafld review` pins a `reviewed_diff` hash over the working tree and `scafld complete` validates the diff still matches. When you complete spec A, scafld moves files (`.ai/specs/active/A.yaml → archive/2026-04/A.yaml`, `.ai/runs/A → .ai/runs/archive/2026-04/A`), which changes the working tree. The next `scafld review B` captures the now-modified tree, and any subsequent reviewer activity that triggers another scafld bookkeeping write breaks B's anchor — even though no engineering code changed. The cause is asymmetric exclusion. `audit_scope.git_sync_excluded_paths()` already excludes `.ai/specs/`, `.ai/reviews/`, `.ai/runs/` from sync-drift, but `review_workflow.review_binding_excluded_rels` only excludes the review file and the active task's own runs directory — not sibling archive moves. Fix the cause: add the same control-plane prefixes to the review-binding exclusion list. The diff that anchors a review should be the engineering diff, not engineering-plus-scafld- bookkeeping.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/review_workflow.py` (87-98) - `review_binding_excluded_rels` builds the exclusion list passed to `capture_review_git_state`. It currently includes only the review markdown for the task and the task's own runs directory. Extend it with the same control-plane prefixes already used by `audit_scope.git_sync_excluded_paths()`.

Invariants:
- `reviewer_diff_anchors_engineering_changes_not_scafld_bookkeeping`
- `sibling_complete_writes_do_not_break_review_anchor`
- `real_engineering_changes_after_review_still_invalidate_anchor`

Related docs:
- `docs/scope-auditing.md`

## Objectives

- Stop scafld's own bookkeeping moves from invalidating review anchors.
- Reuse the existing control-plane exclusion list rather than introducing a parallel one.
- Preserve the existing guard that real engineering changes between review and complete invalidate the anchor.

## Scope



## Dependencies

- None.

## Assumptions

- The `audit_scope.git_sync_excluded_paths()` list is the canonical scafld control-plane prefix list; review-binding can reuse it.
- The current per-task exclusion (review markdown + task runs dir) remains needed for the same-task review-write cycle.

## Touchpoints

- scafld/review_workflow.py: Single function update to extend exclusion list.

## Risks

- A real engineering change that lives under `.ai/` (e.g. operators editing scafld config) would be excluded from the anchor.

## Acceptance

Profile: standard

Definition of done:
- [ ] `dod1` Completing one spec does not invalidate another spec's review anchor.
- [ ] `dod2` Real engineering changes (outside `.ai/`) between review and complete continue to invalidate the anchor.
- [ ] `dod3` Existing scafld test suite stays green.

Validation:
- [ ] `v1` test
  - Command: `.venv/bin/python -m unittest discover tests/`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - New review-anchor exclusion test passes.
  - Command: `.venv/bin/python -m unittest tests.test_review_anchor_exclusion`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Extend review-binding exclusion list

Goal: Update `review_binding_excluded_rels` to include the same control-plane prefixes already used by `git_sync_excluded_paths`, so scafld's own archive moves do not invalidate review anchors.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Existing scafld pytest suite passes under the change.
  - Command: `.venv/bin/python -m unittest discover tests/`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test - New review-anchor exclusion test passes.
  - Command: `.venv/bin/python -m unittest tests.test_review_anchor_exclusion`
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
Timestamp: '2026-04-27T15:32:19Z'
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

- 2026-04-27T15:28:17Z - claude - Drafted under runx flagship-dogfood-wave audit; covers finding 5 (scafld review-gate cascade).
- 2026-04-27T15:29:07Z - cli - Spec approved
- 2026-04-27T15:32:49Z - cli - Spec completed
