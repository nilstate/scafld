---
spec_version: '2.0'
task_id: improve-report-triage
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:32Z'
status: completed
harden_status: not_run
size: small
risk_level: low
---

# Improve report triage output

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

Extend `scafld report` from passive aggregate counts to actionable workflow triage. Surface stale drafts, approved specs that never started, active specs with no exec results, and review drift once review artifacts are bound to git state.

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

- Keep the aggregate report, but add actionable workflow triage.
- Highlight stale and blocked specs without requiring manual digging.
- Reflect review drift once commit binding exists.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Extend report output with workflow triage sections.
- tests/lifecycle_smoke.sh: Optionally use lifecycle/report assertions for actionable output.
- docs/cli-reference.md: Document the new report sections.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` `scafld report` highlights stale drafts and inactive approved/active specs.
- [ ] `dod2` Report can surface review drift once review binding exists.
- [ ] `dod3` Docs describe the new triage sections.

Validation:
- [ ] `v1` test - Lifecycle smoke still passes after report changes.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Add Actionable Report Triage

Goal: Make `scafld report` point operators at specs that need attention.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Lifecycle smoke passes with report assertions.
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
- 2026-04-21T06:15:00Z - assistant - Expanded draft into actionable report triage improvements.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:31:02Z - cli - Execution started
- 2026-04-21T06:56:32Z - cli - Spec completed
