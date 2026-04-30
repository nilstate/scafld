---
spec_version: '2.0'
task_id: modularize-cli-core
created: '2026-04-21T06:11:31Z'
updated: '2026-04-21T06:56:33Z'
status: completed
harden_status: not_run
size: large
risk_level: medium
---

# Modularize CLI core

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

Reduce change risk in the single-file CLI by extracting stable helper layers into the importable `scafld` package. Focus on pure helpers and command-local support code first: config loading, git-state helpers, harden citation helpers, and review/report helpers. Keep CLI behavior unchanged.

## Context

CWD: `.`

Packages:
- `cli`
- `scafld`
- `tests`

Files impacted:
- none

Invariants:
- `domain_boundaries`
- `no_legacy_code`
- `public_api_stable`

Related docs:
- none

## Objectives

- Extract stable helper code from `cli/scafld` into importable modules.
- Shrink the command file without changing CLI behavior.
- Keep smoke coverage green after the extraction.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Retain argument parsing and command dispatch while importing extracted helpers.
- scafld/: New helper modules for config, git state, harden verification, and review/report support.
- tests/*.sh: Existing smoke tests guard behavior during extraction.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Helper code moves into package modules with no CLI behavior regressions.
- [ ] `dod2` `cli/scafld` becomes materially smaller and more command-focused.
- [ ] `dod3` Existing smoke tests remain green after the extraction.

Validation:
- [ ] `v1` test - Smoke suite passes after modularization.
  - Command: `bash tests/harden_smoke.sh && bash tests/review_gate_smoke.sh && bash tests/update_smoke.sh && bash tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Extract Stable Helper Modules

Goal: Move reusable helper layers into the package while keeping the CLI surface stable.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - The smoke suite passes after helper extraction.
  - Command: `bash tests/harden_smoke.sh && bash tests/review_gate_smoke.sh && bash tests/update_smoke.sh && bash tests/package_smoke.sh`
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
Timestamp: '2026-04-21T06:56:33Z'
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
- 2026-04-21T06:15:00Z - assistant - Expanded draft into a helper extraction plan that preserves current CLI behavior.
- 2026-04-21T06:16:06Z - cli - Spec approved
- 2026-04-21T06:31:02Z - cli - Execution started
- 2026-04-21T06:56:33Z - cli - Spec completed
