---
spec_version: '2.0'
task_id: review-gate-reviewed-state-match
created: '2026-04-25T08:22:48Z'
updated: '2026-04-25T08:41:42Z'
status: completed
harden_status: in_progress
size: medium
risk_level: medium
---

# Make review binding honest and review reopen idempotent

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

Fix the review lifecycle so scafld can close real work without tripping over its own control-plane artifacts. Today `review` fingerprints workspace state before it emits review handoffs and session files, so `complete` can fail even when the reviewer followed the documented flow. Re-running `review` while the latest round is still in progress also appends a second round instead of refreshing the active one, which makes dogfood noisy and breaks the one-open-review mental model. This slice should make review binding track the actual reviewed engineering state, ignore scafld-generated review control-plane files, refresh incomplete rounds in place, and prove the flow with smoke coverage that exercises the real CLI.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_workflow.py` (task-selected) - Review-open should bind reviewed state at the right moment and refresh incomplete rounds instead of appending duplicates.
- `scafld/git_state.py` (task-selected) - Reviewed-state capture must support excluding review control-plane paths cleanly.
- `scafld/review_runtime.py` (task-selected) - Review JSON output should reflect whether a round was opened or refreshed.
- `scafld/commands/reporting.py` (task-selected) - Report should use the same reviewed-state semantics as complete when checking drift.
- `tests/review_gate_smoke.sh` (task-selected) - The real CLI smoke should prove review-open to complete works without false drift and that rerunning review refreshes the active round.
- `docs/review.md` (task-selected) - The operator docs should describe the honest review binding model and idempotent reopen behavior.

Invariants:
- `review_gate_is_the_quality_boundary`
- `control_plane_artifacts_do_not_count_as_reviewed_product_changes`
- `one_open_review_round_per_task`
- `human_output_remains_default`
- `json_mode_uses_stable_envelopes`

Related docs:
- `AGENTS.md`
- `CONVENTIONS.md`
- `README.md`
- `docs/review.md`
- `docs/lifecycle.md`

## Objectives

- Bind review rounds to the actual reviewed engineering workspace instead of a pre-handoff snapshot.
- Exclude scafld-generated review control-plane artifacts from reviewed-state drift checks without hiding real product-file changes.
- Make `scafld review` refresh the latest in-progress round instead of appending Review 2 noise.
- Expose the refresh/open distinction in the machine-readable review payload.
- Prove the fixed flow with smoke coverage using the real packaged CLI.

## Scope



## Dependencies

- None.

## Assumptions

- The task-scoped `.ai/runs/{task_id}` tree is control-plane state and should not block completion.
- The review artifact itself should remain excluded from reviewed-state capture.
- Refreshing an incomplete round should replace stale scaffold content rather than silently preserving stale partial findings.

## Touchpoints

- review binding: The reviewed-state contract should mean product files changed after review, not scafld's own handoff/session churn.
- review reopen UX: Operators should see one active round until the challenger finishes it.
- machine output: JSON review output should tell automation whether a round was opened or refreshed.

## Risks

- Over-broad exclusions could let real engineering changes slip past the review gate.
- Refreshing an in-progress round could corrupt the review file if the replacement logic is loose.
- Report/status drift checks could diverge from complete again if they keep separate exclusion logic.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` A freshly opened review round can be completed after the challenger fills it in, without false drift from scafld-generated task artifacts.
- [ ] `dod2` Re-running `scafld review` while the latest round is still in progress refreshes that round in place.
- [ ] `dod3` Review drift checks share one exclusion model across review, complete, and report.
- [ ] `dod4` Review JSON output exposes whether the latest round was opened or refreshed.
- [ ] `dod5` Review docs and smoke tests describe and prove the new lifecycle behavior.

Validation:
- [ ] `v1` compile - Compile the Python sources after the review lifecycle changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke proves honest binding and idempotent reopen behavior.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Lifecycle smoke still passes with the refreshed review semantics.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Bind review state to the real reviewed workspace

Goal: Make reviewed-state capture ignore task control-plane churn while still detecting real product drift.

Status: completed
Dependencies: none

Changes:
- `scafld/review_workflow.py` (all) - Centralize the review-binding exclusions, materialize review control-plane files before the binding snapshot is finalized, and reuse the same capture semantics in gate evaluation.
- `scafld/git_state.py` (all) - Support reviewed-state capture with explicit multiple exclusions rather than a single review file path.
- `scafld/commands/reporting.py` (all) - Use the same shared reviewed-state exclusion rules when reporting review drift.
- `tests/review_gate_smoke.sh` (all) - Add a smoke case that opens a real review round, fills it in, and completes successfully without false drift from scafld-generated run artifacts.

Acceptance:
- [x] `ac1_1` integration - Review gate smoke covers a clean review-open to complete flow.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Refresh incomplete review rounds in place

Goal: Keep one active review round per task until the challenger finishes it.

Status: completed
Dependencies: phase1

Changes:
- `scafld/review_workflow.py` (all) - Replace the latest in-progress review round instead of appending a new one when `scafld review` is rerun.
- `scafld/review_runtime.py` (all) - Expose whether review opened a new round or refreshed the current one.
- `tests/review_gate_smoke.sh` (all) - Assert that rerunning review preserves one round and refreshes the scaffold instead of appending Review 2.

Acceptance:
- [x] `ac2_1` integration - Review gate smoke proves rerunning review refreshes the active round in place.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Document and prove the final review flow

Goal: Make the new review model visible to operators and keep it locked with the existing lifecycle suite.

Status: completed
Dependencies: phase2

Changes:
- `docs/review.md` (all) - Document the honest review-binding model, the control-plane exclusions, and the fact that `review` refreshes an incomplete round.
- `tests/lifecycle_smoke.sh` (all) - Keep the broader lifecycle smoke aligned with the refreshed review lifecycle semantics.

Acceptance:
- [x] `ac3_1` compile - Python sources still compile.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Lifecycle smoke passes with the new review behavior.
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
Timestamp: '2026-04-25T08:41:09Z'
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

- 2026-04-25T08:22:48Z - user - Spec created via scafld plan
- 2026-04-25T09:15:00Z - assistant - Replaced the placeholder draft with a bounded review-lifecycle spec grounded in live dogfood and the current review gate code paths.
- 2026-04-25T08:31:37Z - cli - Spec approved
- 2026-04-25T08:31:43Z - cli - Execution started
- 2026-04-25T08:41:42Z - cli - Spec completed
