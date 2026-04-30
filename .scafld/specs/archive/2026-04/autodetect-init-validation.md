---
spec_version: '2.0'
task_id: autodetect-init-validation
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:32Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Autodetect init validation commands

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

Make `scafld init` produce a more useful `.ai/config.local.yaml` by inspecting the current repo for common Node and Python toolchains and pre-populating suggested build, test, lint, and typecheck commands instead of generic placeholders. Fall back gracefully when the repo shape is unknown.

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

- Detect common Node and Python project markers during `scafld init`.
- Emit better starter commands in `.ai/config.local.yaml`.
- Keep unknown repos on a safe placeholder fallback.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Add repo inspection helpers and use them when writing config.local.
- tests/update_smoke.sh: Cover generated config output for detected and fallback cases.
- README.md: Document that init now suggests commands from common project layouts.
- docs/installation.md: Explain repo inspection and fallback behavior during init.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` `scafld init` detects common Node/Python markers and writes concrete starter commands.
- [ ] `dod2` Unknown repos still get a safe placeholder fallback.
- [ ] `dod3` Smoke coverage and docs reflect the new init behavior.

Validation:
- [ ] `v1` test - Update smoke covers init autodetection output.
  - Command: `bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Detect Common Repo Toolchains During Init

Goal: Generate a more useful `.ai/config.local.yaml` from observed repo markers.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Update smoke passes with init autodetection fixtures.
  - Command: `bash tests/update_smoke.sh`
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
- 2026-04-21T06:15:00Z - assistant - Expanded draft into init-time repo detection and starter command generation.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:31:02Z - cli - Execution started
- 2026-04-21T06:56:32Z - cli - Spec completed
