---
spec_version: '2.0'
task_id: add-lifecycle-e2e-smoke
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:32Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Add lifecycle end-to-end smoke test

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

Add a true workflow smoke test that exercises the core lifecycle on a tiny fixture repo: `init -> new -> validate -> approve -> start -> exec -> review -> complete`. This should verify that the commands continue to compose after recent changes to harden, review gating, and reporting.

## Context

CWD: `.`

Packages:
- `tests`
- `cli`
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

- Exercise the happy-path lifecycle end to end on a disposable git repo.
- Verify that exec, review, and complete still compose cleanly.
- Document the coverage so future changes treat it as a core workflow check.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- tests/lifecycle_smoke.sh: New end-to-end lifecycle smoke coverage.
- .github/workflows/review-gate-smoke.yml: Run the new lifecycle smoke in CI.
- docs/quickstart.md: Reference the lifecycle smoke as an integrity check.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` A single smoke test covers the lifecycle from init to complete.
- [ ] `dod2` CI runs the lifecycle smoke alongside existing smoke coverage.
- [ ] `dod3` Docs mention the new end-to-end workflow check.

Validation:
- [ ] `v1` test - Lifecycle smoke passes locally.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Exercise The Full Lifecycle In One Smoke Test

Goal: Add a disposable repo smoke test that covers the core workflow.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Lifecycle smoke passes locally.
  - Command: `bash tests/lifecycle_smoke.sh`
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
Review rounds: 4
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
- 2026-04-21T06:15:00Z - assistant - Expanded draft into an end-to-end lifecycle smoke test.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:31:02Z - cli - Execution started
- 2026-04-21T06:56:32Z - cli - Spec completed
