---
spec_version: '2.0'
task_id: reduce-core-brittleness
created: '2026-04-21T12:20:22Z'
updated: '2026-04-21T12:47:26Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Reduce core brittleness

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

Finish the next cleanup pass on scafld's command core so the tool feels tighter under both human use and automation. The biggest remaining brittleness is not feature coverage anymore; it is the residue left in `cli/scafld` after the earlier extraction round. `cmd_new` still builds its repo-aware template inline, projection/origin helpers still live in the CLI, review parsing is still implemented there instead of in the review module, and the shell smoke tests duplicate fragile JSON assertion glue. This round extracts those residual helper clusters into modules, gives the smoke tests one shared assertion harness, and replaces ad hoc stringly error-code usage with a first-class catalog so future integrations do not keep copying literals around.

## Context

CWD: `.`

Packages:
- `cli`
- `scafld`
- `tests`
- `docs`

Files impacted:
- none

Invariants:
- `human_cli_behavior_stays_intact`
- `machine_callers_never_need_terminal_scraping`
- `shared_helpers_live_outside_the_entrypoint`
- `json_mode_uses_stable_envelopes`
- `review_and_projection_surfaces_remain_deterministic`

Related docs:
- none

## Objectives

- Move residual template/origin/projection/review helper logic out of `cli/scafld` into reusable modules.
- Give shell smoke tests one shared JSON assertion path instead of repeating inline Python eval blocks.
- Define a single error-code catalog that command code and JSON output can reuse without string drift.
- Keep the current JSON and human contracts stable while making the internals easier to extend.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Trim the entrypoint back to command flow and thin composition over extracted helpers.
- scafld/spec_templates.py: Own repo-aware draft template generation for `scafld new`.
- scafld/projections.py: Own origin metadata shaping plus projection/model helper logic.
- scafld/reviewing.py: Own review artifact parsing instead of leaving it embedded in the CLI.
- scafld/error_codes.py: Provide one shared source of truth for structured command error codes.
- tests/smoke_lib.sh: Provide shared shell smoke helpers for capture, assertions, and temp-dir cleanup.
- tests/json_assert.py: Provide one reusable JSON assertion helper for shell smoke suites.
- package.json: Ensure npm packaging continues to ship the Python runtime modules the CLI imports.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Residual template/origin/projection/review helpers are extracted out of `cli/scafld`.
- [x] `dod2` Smoke suites reuse shared helper code instead of repeating inline JSON assertion blocks.
- [x] `dod3` Structured error codes are centralized and callers no longer rely on scattered string literals.
- [x] `dod4` Lifecycle, projection, git-origin, harden, and review-gate behavior remains stable.

Validation:
- [ ] `v1` test - Focused unit coverage for runtime/spec/git modules still passes.
  - Command: `python3 -m unittest tests.test_command_runtime tests.test_spec_store tests.test_git_state`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - JSON contract smoke passes through the shared assertion harness.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Git-bound origin smoke still passes after helper extraction.
  - Command: `./tests/git_origin_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Projection surface smoke still passes after projection helper extraction.
  - Command: `./tests/projection_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` integration - Harden smoke still passes with shared shell helpers.
  - Command: `./tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v6` integration - Review gate smoke still passes with the shared JSON assertion harness.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v7` integration - Package smoke still proves the published wheel and npm tarball contain the required runtime surface.
  - Command: `./tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Extract Residual Core Helpers

Goal: Move remaining template, projection, origin, and review helper logic into reusable modules.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Focused unit coverage still passes after helper extraction.
  - Command: `python3 -m unittest tests.test_command_runtime tests.test_spec_store tests.test_git_state`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Stabilize Smoke Harness

Goal: Give shell smokes one shared assertion and cleanup layer instead of repeated inline helpers.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` integration - JSON contract smoke passes through the shared harness.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - Review gate smoke passes through the shared harness.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Centralize Error Codes

Goal: Replace scattered string literals with one reusable error-code catalog.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac3_1` integration - Projection and git-origin smokes still emit the expected structured error codes.
  - Command: `./tests/git_origin_smoke.sh && ./tests/projection_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Harden and review-gate smokes still emit stable structured error codes.
  - Command: `./tests/harden_smoke.sh && ./tests/review_gate_smoke.sh`
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
Timestamp: '2026-04-21T12:47:02Z'
Review rounds: 2
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

- 2026-04-21T12:20:22Z - user - Spec created via scafld new
- 2026-04-21T13:10:00Z - assistant - Expanded the draft into a focused cleanup round covering residual CLI helper extraction, shared smoke assertions, and centralized error codes.
- 2026-04-21T12:26:43Z - cli - Spec approved
- 2026-04-21T12:26:49Z - cli - Execution started
- 2026-04-21T12:46:30Z - assistant - Extracted residual template/projection/review helpers into modules, unified smoke helpers behind one shell library and JSON assertion tool, tightened npm runtime packaging, and passed the defined execution checks.
- 2026-04-21T12:47:26Z - cli - Spec completed
