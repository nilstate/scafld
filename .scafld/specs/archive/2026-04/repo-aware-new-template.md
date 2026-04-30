---
spec_version: '2.0'
task_id: repo-aware-new-template
created: '2026-04-21T11:40:22Z'
updated: '2026-04-21T11:45:45Z'
status: completed
harden_status: not_run
size: small
risk_level: medium
---

# Make new templates repo-aware

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

Make `scafld new` generate a materially better first draft by reusing the repo-shape knowledge that `scafld init` already has. Right now `new` ignores detected toolchain context and emits a wall of generic TODOs, including placeholder commands, even in repos where scafld can already suggest concrete validation commands. This round should keep drafts intentionally incomplete, but replace the most generic placeholders with repo-aware prompts and effective validation commands.

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
- `human_output_remains_default`
- `json_mode_uses_stable_envelopes`
- `drafts_stay_fail_closed_until_humans_edit_them`

Related docs:
- none

## Objectives

- Make `scafld new` reuse detected repo context and validation commands.
- Keep generated drafts intentionally invalid until the operator fills in the task-specific details.
- Prove the new template behavior in existing smoke coverage.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Generate repo-aware template text and concrete validation commands in `scafld new`.
- tests/update_smoke.sh: Assert that generated drafts inherit concrete commands and contextual prompts.
- docs/cli-reference.md: Document that `new` reuses repo-aware defaults when available.
- docs/quickstart.md: Show that the first draft now includes repo-aware validation commands.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Generated drafts include concrete validation commands when repo detection can suggest them.
- [x] `dod2` Generated drafts still contain contextual TODO prompts for human-owned task details.
- [x] `dod3` Smoke coverage proves both repo-aware and fallback draft generation.

Validation:
- [ ] `v1` integration - Update smoke covers repo-aware `scafld new` defaults.
  - Command: `bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Generate Better First Drafts

Goal: Teach `scafld new` to scaffold drafts with repo-aware prompts and concrete validation commands.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` integration - Update smoke passes with repo-aware `new` template assertions.
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
Timestamp: '2026-04-21T11:45:23Z'
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

- 2026-04-21T11:40:22Z - user - Spec created via scafld new
- 2026-04-21T11:42:00Z - assistant - Expanded the draft into a repo-aware `scafld new` slice that reuses detection without pretending to know task-specific scope.
- 2026-04-21T11:41:02Z - cli - Spec approved
- 2026-04-21T11:41:09Z - cli - Execution started
- 2026-04-21T11:45:45Z - cli - Spec completed
