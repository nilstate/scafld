---
spec_version: '2.0'
task_id: bind-review-to-commit
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:32Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Bind review artifacts to commits

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

Strengthen the review gate by recording git state in Review Artifact v3 and refusing normal completion when the current workspace no longer matches what was reviewed. The latest review round should capture the reviewed `HEAD`, whether the tree was dirty, and enough information for `scafld complete` to detect drift. Human override remains the escape hatch.

## Context

CWD: `.`

Packages:
- `cli`
- `tests`
- `docs`

Files impacted:
- none

Invariants:
- `domain_boundaries`
- `no_legacy_code`
- `public_api_stable`

Related docs:
- none

## Objectives

- Record reviewed git state in review metadata.
- Block `scafld complete` when the current repo no longer matches the reviewed state.
- Preserve the existing human-reviewed override path.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Capture git state during review, validate metadata, and enforce commit binding during completion.
- tests/review_gate_smoke.sh: Exercise matched and mismatched review state scenarios.
- docs/review.md: Document reviewed commit binding and override semantics.
- docs/cli-reference.md: Document completion blocking when review and HEAD diverge.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Review metadata records reviewed git state.
- [ ] `dod2` `scafld complete` blocks if the current git state no longer matches the review.
- [ ] `dod3` The human override path still works and is auditable.

Validation:
- [ ] `v1` test - Review gate smoke covers commit binding.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Capture And Enforce Reviewed Git State

Goal: Bind review artifacts to a concrete git state and enforce it at completion.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Review gate smoke passes with commit binding enabled.
  - Command: `bash tests/review_gate_smoke.sh`
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
Timestamp: '2026-04-21T06:56:32Z'
Review rounds: 5
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

- 2026-04-21T06:11:31Z - user - Spec created via scafld new
- 2026-04-21T06:15:00Z - assistant - Expanded draft into review metadata binding and completion drift enforcement.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:18:22Z - cli - Execution started
- 2026-04-21T06:56:32Z - cli - Spec completed
