---
spec_version: '2.0'
task_id: smoke-ci-workflow
created: '2026-03-26T09:05:13Z'
updated: '2026-03-26T09:08:46Z'
status: completed
harden_status: not_run
size: small
risk_level: low
---

# Add smoke CI workflow and badge

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

Add a GitHub Actions workflow that runs the Scafld review-gate smoke matrix on pushes and pull requests, make the smoke harness portable outside the current workstation path, and surface the workflow status with a README badge.

## Context

CWD: `.`

Packages:
- `.github/workflows`
- `tests`
- `.`

Files impacted:
- none

Invariants:
- `domain_boundaries`

Related docs:
- none

## Objectives

- Run the review-gate smoke suite in GitHub Actions
- Keep the smoke harness runnable both locally and in CI
- Expose workflow status in the Scafld README

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- Smoke harness portability: The smoke script currently hardcodes a workstation-specific CLI path and must derive it from the repo layout or an override
- GitHub Actions: Add a workflow that runs the same local smoke matrix used during development
- README surface: Add a status badge near the project title so workflow health is visible

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` The smoke harness derives the CLI path portably so it works in CI
- [ ] `dod2` GitHub Actions runs the smoke matrix on push and pull_request
- [ ] `dod3` README exposes the workflow status badge

Validation:
- [ ] `v1` test - Smoke harness shell syntax is valid
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - CLI still parses after the harness portability change
  - Command: `PYTHONDONTWRITEBYTECODE=1 python3 -m py_compile cli/scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Full smoke matrix passes locally with the new harness path logic
  - Command: `./tests/review_gate_smoke.sh all`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Workflow and README reference the smoke CI path
  - Command: `rg -n 'review-gate-smoke|tests/review_gate_smoke\.sh all|badge\.svg' README.md .github/workflows/review-gate-smoke.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Add smoke CI workflow

Goal: Make the smoke harness runnable in CI, add the workflow, and surface its status in the README.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Smoke harness shell syntax is valid
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test - CLI still parses after the harness portability change
  - Command: `PYTHONDONTWRITEBYTECODE=1 python3 -m py_compile cli/scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` test - Full smoke matrix passes locally
  - Command: `./tests/review_gate_smoke.sh all`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_4` test - Workflow and badge are wired to the smoke CI path
  - Command: `rg -n 'review-gate-smoke|tests/review_gate_smoke\.sh all|badge\.svg' README.md .github/workflows/review-gate-smoke.yml`
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
Timestamp: '2026-03-26T09:08:10Z'
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
- `id`: adversarial_review

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

- 2026-03-26T09:05:13Z - user - Spec created via scafld new
- 2026-03-26T09:06:00Z - agent - Scoped the change to a portable smoke harness, one GitHub Actions workflow, and a README badge
- 2026-03-26T09:06:14Z - cli - Spec approved
- 2026-03-26T09:06:32Z - cli - Execution started
- 2026-03-26T09:08:46Z - cli - Spec completed
