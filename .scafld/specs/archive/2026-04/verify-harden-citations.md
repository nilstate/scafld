---
spec_version: '2.0'
task_id: verify-harden-citations
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:01Z'
status: completed
harden_status: not_run
size: small
risk_level: low
---

# Verify harden citations

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

Add warning-only verification for `harden_rounds[*].questions[*].grounded_in` so `scafld harden --mark-passed` can catch cheap fake citations without turning harden into a gate. `code:<file>:<line>` entries should warn when the file does not exist or the line is out of bounds. `archive:<task_id>` entries should warn when no archived spec exists. The command should still mark the round passed.

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

- Warn when harden citations are structurally valid but unresolvable.
- Keep `scafld harden --mark-passed` non-blocking.
- Document the warning behavior and cover it in smoke tests.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Add citation resolution helpers and emit warnings during `--mark-passed`.
- tests/harden_smoke.sh: Add fake citation fixtures and assert warning-only behavior.
- docs/cli-reference.md: Document that citation verification warns but does not block.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Fake `code:` and `archive:` citations produce warnings during `--mark-passed`.
- [ ] `dod2` `--mark-passed` still succeeds and closes the round after warnings.
- [ ] `dod3` Smoke tests and docs reflect the warning-only contract.

Validation:
- [ ] `v1` test - Harden smoke covers citation verification.
  - Command: `bash tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Warn On Unresolvable Harden Citations

Goal: Teach `scafld harden --mark-passed` to verify and warn on fake citations.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Harden smoke passes with citation verification enabled.
  - Command: `bash tests/harden_smoke.sh`
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
Timestamp: '2026-04-21T06:56:01Z'
Review rounds: 3
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
- 2026-04-21T06:15:00Z - assistant - Expanded draft into a warning-only citation verification change for harden.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:16:33Z - cli - Execution started
- 2026-04-21T06:56:01Z - cli - Spec completed
