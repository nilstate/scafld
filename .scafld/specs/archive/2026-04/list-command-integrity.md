---
spec_version: '2.0'
task_id: list-command-integrity
created: '2026-04-25T08:52:38Z'
updated: '2026-04-25T08:59:38Z'
status: completed
harden_status: in_progress
size: small
risk_level: low
---

# Restore list command runtime integrity

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

Fix the current regression where `scafld list` crashes with a `NameError` instead of showing the task inventory. The immediate bug is simple: `cmd_list()` calls `yaml_read_field()` but `scafld/commands/lifecycle.py` no longer imports it. The real product requirement is a little broader: `list` is one of the default control-surface commands, so it needs a direct regression test that exercises the packaged CLI across the normal spec states instead of relying on incidental coverage from other flows. This task restores the command, adds a real smoke, and keeps the scope tight.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/commands/lifecycle.py` (task-selected) - The list command currently references `yaml_read_field()` without importing it.
- `tests/list_command_smoke.sh` (all) - A dedicated smoke should prove `scafld list` works through the packaged CLI and respects basic filters.

Invariants:
- `default_control_surface_commands_must_not_crash`
- `packaged_cli_is_the_truth_surface`
- `list_output_remains_human_first`

Related docs:
- `README.md`
- `docs/lifecycle.md`
- `docs/cli-reference.md`

## Objectives

- Make `scafld list` run successfully again from the packaged CLI.
- Add a dedicated smoke that covers the normal lifecycle buckets and filter behavior.
- Keep the fix narrow and avoid changing list semantics beyond restoring integrity.

## Scope



## Dependencies

- None.

## Assumptions

- find_all_specs() already provides the right source of truth for list inventory.
- A direct smoke is the right guardrail because this bug bypassed the current suites.

## Touchpoints

- command integrity: scafld list should be as reliable as status and report.
- regression coverage: The packaged CLI should be exercised directly instead of relying on helper-level tests.

## Risks

- A too-minimal fix could restore the import but still leave list filters untested and brittle.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` scafld list no longer crashes and shows existing specs.
- [ ] `dod2` A dedicated smoke proves the packaged CLI can list lifecycle buckets and apply a basic filter.

Validation:
- [ ] `v1` compile - Compile the Python sources after the list command fix.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Dedicated list-command smoke passes.
  - Command: `bash tests/list_command_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Restore list command execution

Goal: Make the default list surface run again without changing its human-first contract.

Status: completed
Dependencies: none

Changes:
- `scafld/commands/lifecycle.py` (all) - Import the missing scalar YAML reader and keep list inventory reading aligned with the existing lifecycle parsing helpers.

Acceptance:
- [x] `ac1_1` compile - Python sources compile after the list command fix.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Restore the live command and lock it with a smoke

Goal: Land the runtime fix and prove the command works through the real CLI across lifecycle buckets and a basic filter.

Status: completed
Dependencies: phase1

Changes:
- `scafld/commands/lifecycle.py` (all) - Restore the live list command path so the packaged CLI can enumerate specs without crashing.
- `tests/list_command_smoke.sh` (all) - Create a packaged-CLI smoke that seeds a few specs, exercises `scafld list` with and without filters, and asserts the command does not crash.

Acceptance:
- [x] `ac2_1` integration - Dedicated list-command smoke passes.
  - Command: `bash tests/list_command_smoke.sh`
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
Timestamp: '2026-04-25T09:00:15Z'
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

- 2026-04-25T08:52:38Z - user - Spec created via scafld plan
- 2026-04-25T09:03:00Z - assistant - Replaced the placeholder draft with a bounded spec for the live scafld list NameError regression and its direct CLI smoke coverage.
- 2026-04-25T08:55:27Z - cli - Spec approved
- 2026-04-25T08:55:35Z - cli - Execution started
- 2026-04-25T08:57:30Z - assistant - Tightened phase2 after phase1 compile passed without exercising the broken user-facing command; phase2 now carries the runtime fix plus the direct CLI smoke.
- 2026-04-25T08:59:38Z - cli - Spec completed
